use portable_pty::{native_pty_system, CommandBuilder, PtySize, PtySystem};
use std::collections::HashMap;
use std::io::{Read, Write};
use std::sync::{Arc, Mutex};
use tauri::AppHandle;
use tauri::Emitter;

pub struct PtySession {
    pub writer: Box<dyn Write + Send>,
    pub running: Arc<Mutex<bool>>,
    pub master: Arc<Mutex<Box<dyn portable_pty::MasterPty + Send>>>,
}

pub struct PtyManager {
    sessions: HashMap<String, PtySession>,
    pty_system: Box<dyn PtySystem + Send>,
}

impl PtyManager {
    pub fn new() -> Self {
        Self {
            sessions: HashMap::new(),
            pty_system: native_pty_system(),
        }
    }

    pub fn spawn(
        &mut self,
        id: String,
        app: AppHandle,
        shell: Option<String>,
    ) -> Result<(), String> {
        if self.sessions.contains_key(&id) {
            return Ok(());
        }

        let pair = self
            .pty_system
            .openpty(PtySize {
                rows: 24,
                cols: 80,
                pixel_width: 0,
                pixel_height: 0,
            })
            .map_err(|e| e.to_string())?;

        let shell_path = shell.unwrap_or_else(|| {
            if cfg!(target_os = "windows") {
                "powershell.exe".to_string()
            } else if cfg!(target_os = "macos") {
                std::env::var("SHELL").unwrap_or_else(|_| "/bin/zsh".to_string())
            } else {
                std::env::var("SHELL").unwrap_or_else(|_| "/bin/bash".to_string())
            }
        });

        let mut cmd = CommandBuilder::new(&shell_path);
        cmd.cwd(dirs::home_dir().unwrap_or_default());

        let mut child = pair
            .slave
            .spawn_command(cmd)
            .map_err(|e| e.to_string())?;

        let mut reader = pair
            .master
            .try_clone_reader()
            .map_err(|e| e.to_string())?;
        let writer = pair
            .master
            .take_writer()
            .map_err(|e| e.to_string())?;

        let running = Arc::new(Mutex::new(true));
        let running_clone = running.clone();
        let id_clone = id.clone();
        let app_clone = app.clone();

        std::thread::spawn(move || {
            let mut buf = [0u8; 8192];
            while *running_clone.lock().unwrap() {
                match reader.read(&mut buf) {
                    Ok(0) => break,
                    Ok(n) => {
                        let data = buf[..n].to_vec();
                        let _ = app_clone.emit("pty-output", (id_clone.clone(), data));
                    }
                    Err(_) => break,
                }
            }
        });

        let running_child = running.clone();
        std::thread::spawn(move || {
            let _ = child.wait();
            *running_child.lock().unwrap() = false;
        });

        self.sessions.insert(
            id.clone(),
            PtySession {
                writer: Box::new(writer),
                running,
                master: Arc::new(Mutex::new(pair.master)),
            },
        );

        Ok(())
    }

    pub fn write(&mut self, id: &str, data: &[u8]) -> Result<(), String> {
        if let Some(session) = self.sessions.get_mut(id) {
            if *session.running.lock().unwrap() {
                session
                    .writer
                    .write_all(data)
                    .map_err(|e| e.to_string())?;
                session.writer.flush().map_err(|e| e.to_string())?;
            }
        }
        Ok(())
    }

    pub fn resize(&mut self, id: &str, rows: u16, cols: u16) -> Result<(), String> {
        if let Some(session) = self.sessions.get(id) {
            let master = session.master.lock().map_err(|e| e.to_string())?;
            master
                .resize(PtySize {
                    rows,
                    cols,
                    pixel_width: 0,
                    pixel_height: 0,
                })
                .map_err(|e| e.to_string())?;
        }
        Ok(())
    }

    pub fn close(&mut self, id: &str) -> Result<(), String> {
        if let Some(session) = self.sessions.remove(id) {
            *session.running.lock().unwrap() = false;
        }
        Ok(())
    }
}
