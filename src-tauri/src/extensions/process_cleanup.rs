//! Cleanup primitives for short-lived extension/provider processes.
//!
//! Provider entry points are often scripts. A direct child kill would leave a
//! script's grandchildren behind, so Unix providers run in a private process
//! group and the whole group is terminated whenever the operation is aborted.

use std::process::Output;
use std::time::Duration;
use tokio::io::AsyncReadExt;
use tokio::process::{Child, Command};

/// Put an extension command in a private process group on Unix.
///
/// Windows has no equivalent configured here; `kill_on_drop(true)` and the
/// explicit child kill/wait paths still cover direct children there.
pub(crate) fn configure_command(command: &mut Command) {
    #[cfg(unix)]
    command.process_group(0);
    #[cfg(not(unix))]
    let _ = command;
}

#[derive(Debug)]
pub(crate) enum CommandOutputError {
    TimedOut(Duration),
    Failed(String),
}

impl std::fmt::Display for CommandOutputError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::TimedOut(timeout) => write!(
                formatter,
                "Command timed out after {} ms",
                timeout.as_millis()
            ),
            Self::Failed(error) => formatter.write_str(error),
        }
    }
}

/// Run a command while retaining both output streams under a timeout.
///
/// The child is spawned explicitly instead of using `Command::output()` so a
/// timeout can terminate and reap it before the future returns. Callers can
/// apply their own output-size limits to the returned buffers.
pub(crate) async fn command_output(
    mut command: Command,
    timeout: Duration,
) -> Result<Output, CommandOutputError> {
    configure_command(&mut command);
    command.kill_on_drop(true);
    let mut child = command
        .spawn()
        .map_err(|error| CommandOutputError::Failed(format!("Cannot spawn command: {error}")))?;
    let mut cleanup = ChildCleanup::new(&child);
    let stdout = child.stdout.take();
    let stderr = child.stderr.take();
    let stdout_task = tokio::spawn(async move {
        let mut bytes = Vec::new();
        match stdout {
            Some(mut stdout) => stdout.read_to_end(&mut bytes).await.map(|_| bytes),
            None => Ok(bytes),
        }
    });
    let stderr_task = tokio::spawn(async move {
        let mut bytes = Vec::new();
        match stderr {
            Some(mut stderr) => stderr.read_to_end(&mut bytes).await.map(|_| bytes),
            None => Ok(bytes),
        }
    });
    let status = match tokio::time::timeout(timeout, child.wait()).await {
        Ok(Ok(status)) => {
            cleanup.finish_after_wait();
            status
        }
        Ok(Err(error)) => {
            cleanup.kill_and_reap(&mut child).await;
            stdout_task.abort();
            stderr_task.abort();
            return Err(CommandOutputError::Failed(format!(
                "Cannot wait for command: {error}"
            )));
        }
        Err(_) => {
            cleanup.kill_and_reap(&mut child).await;
            let _ = stdout_task.await;
            let _ = stderr_task.await;
            return Err(CommandOutputError::TimedOut(timeout));
        }
    };
    let stdout = stdout_task
        .await
        .map_err(|error| {
            CommandOutputError::Failed(format!("Command stdout task failed: {error}"))
        })?
        .map_err(|error| {
            CommandOutputError::Failed(format!("Cannot read command stdout: {error}"))
        })?;
    let stderr = stderr_task
        .await
        .map_err(|error| {
            CommandOutputError::Failed(format!("Command stderr task failed: {error}"))
        })?
        .map_err(|error| {
            CommandOutputError::Failed(format!("Cannot read command stderr: {error}"))
        })?;
    Ok(Output {
        status,
        stdout,
        stderr,
    })
}

/// Tracks a spawned process group until its direct child has been reaped.
pub(crate) struct ChildCleanup {
    #[cfg(unix)]
    pid: Option<u32>,
    armed: bool,
}

impl ChildCleanup {
    pub(crate) fn new(child: &Child) -> Self {
        #[cfg(not(unix))]
        let _ = child;
        Self {
            #[cfg(unix)]
            pid: child.id(),
            armed: true,
        }
    }

    /// Kill the process group (where supported), kill the direct child, and
    /// wait for the direct child so it cannot become a zombie. A process that
    /// already exited is an expected race and all kill errors are ignored.
    pub(crate) async fn kill_and_reap(&mut self, child: &mut Child) {
        self.kill_group();
        let _ = child.kill().await;
        let _ = child.wait().await;
        self.kill_group();
        self.armed = false;
    }

    /// Mark a normally waited child complete, while also removing any
    /// grandchildren that kept the process group alive.
    pub(crate) fn finish_after_wait(&mut self) {
        self.kill_group();
        self.armed = false;
    }

    #[cfg(unix)]
    fn kill_group(&self) {
        if let Some(pid) = self.pid {
            // `process_group(0)` makes the child's PID its PGID. SIGKILL is
            // intentional: providers may trap or ignore softer signals.
            unsafe {
                let _ = libc::killpg(pid as libc::pid_t, libc::SIGKILL);
            }
        }
    }

    #[cfg(unix)]
    fn reap_direct(&self) {
        if let Some(pid) = self.pid {
            // Keep direct-child cleanup independent of process-group
            // support (or a restricted sandbox returning EPERM).
            unsafe { libc::kill(pid as libc::pid_t, libc::SIGKILL) };
            let mut status = 0;
            loop {
                let result = unsafe { libc::waitpid(pid as libc::pid_t, &mut status, 0) };
                if result == pid as libc::pid_t {
                    break;
                }
                let error = std::io::Error::last_os_error();
                if error.raw_os_error() != Some(libc::EINTR) {
                    // ECHILD means another waiter already reaped the process.
                    break;
                }
            }
        }
    }

    #[cfg(not(unix))]
    fn kill_group(&self) {}

    #[cfg(not(unix))]
    fn reap_direct(&self) {}
}

impl Drop for ChildCleanup {
    fn drop(&mut self) {
        if self.armed {
            // Future cancellation can run Drop while the child is being
            // awaited. Tokio's kill_on_drop handles the direct child; this
            // synchronous group kill covers script grandchildren as well.
            self.kill_group();
            // Tokio intentionally does not wait in Child's Drop. Reap here so
            // cancellation cannot accumulate zombies on Unix.
            self.reap_direct();
        }
    }
}
