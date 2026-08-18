use serde::{Deserialize, Serialize};
use std::path::Path;
use std::process::Command;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum PlatformOs {
    Darwin,
    Linux,
    Windows,
}

impl PlatformOs {
    pub(crate) fn as_str(self) -> &'static str {
        match self {
            Self::Darwin => "darwin",
            Self::Linux => "linux",
            Self::Windows => "windows",
        }
    }
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum PlatformArch {
    Arm64,
    X64,
    Armv7,
}

impl PlatformArch {
    pub fn parse(value: &str) -> Result<Self, String> {
        match value.trim().to_ascii_lowercase().as_str() {
            "aarch64" | "arm64" => Ok(Self::Arm64),
            "x86_64" | "amd64" | "x64" => Ok(Self::X64),
            "armv7" | "armv7l" | "armhf" => Ok(Self::Armv7),
            "arm" => Err("Ambiguous ARM architecture: ARM version is required".to_string()),
            other => Err(format!("Unsupported extension architecture: {other}")),
        }
    }

    pub fn canonical_name(self) -> &'static str {
        match self {
            Self::Arm64 => "aarch64",
            Self::X64 => "x86_64",
            Self::Armv7 => "armv7",
        }
    }
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum PlatformLibc {
    Gnu,
    Musl,
    Unknown,
}

impl PlatformLibc {
    fn as_str(self) -> &'static str {
        match self {
            Self::Gnu => "gnu",
            Self::Musl => "musl",
            Self::Unknown => "unknown",
        }
    }
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum PlatformAbi {
    Eabi,
    Eabihf,
    Unknown,
}

impl PlatformAbi {
    fn as_str(self) -> &'static str {
        match self {
            Self::Eabi => "eabi",
            Self::Eabihf => "eabihf",
            Self::Unknown => "unknown",
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct PlatformTarget {
    pub os: PlatformOs,
    pub arch: PlatformArch,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub libc: Option<PlatformLibc>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub abi: Option<PlatformAbi>,
}

impl PlatformTarget {
    pub const fn new(os: PlatformOs, arch: PlatformArch) -> Self {
        Self {
            os,
            arch,
            libc: None,
            abi: None,
        }
    }

    pub fn current() -> Result<Self, String> {
        let os = match std::env::consts::OS {
            "macos" => PlatformOs::Darwin,
            "linux" => PlatformOs::Linux,
            "windows" => PlatformOs::Windows,
            other => return Err(format!("Unsupported extension platform: {other}")),
        };
        let arch = detect_current_arch(os)?;
        if os != PlatformOs::Linux {
            return Ok(Self::new(os, arch));
        }

        let evidence = LinuxPlatformEvidence::collect(arch);
        Ok(Self {
            os,
            arch,
            libc: Some(evidence.libc()),
            abi: (arch == PlatformArch::Armv7).then(|| evidence.arm_abi()),
        })
    }

    pub fn identifier(&self) -> String {
        let mut identifier = format!("{}-{}", self.os.as_str(), self.arch.canonical_name());
        if self.os == PlatformOs::Linux {
            identifier.push('-');
            identifier.push_str(self.libc.unwrap_or(PlatformLibc::Unknown).as_str());
            if self.arch == PlatformArch::Armv7 {
                identifier.push('-');
                identifier.push_str(self.abi.unwrap_or(PlatformAbi::Unknown).as_str());
            }
        }
        identifier
    }

    pub fn package_identifiers(&self) -> Vec<String> {
        vec![self.identifier()]
    }

    pub fn override_identifiers(&self) -> Vec<String> {
        vec![self.identifier(), self.os_identifier()]
    }

    pub fn os_identifier(&self) -> String {
        format!("{}-any", self.os.as_str())
    }

    pub(crate) fn os_name(&self) -> &'static str {
        self.os.as_str()
    }
}

fn detect_current_arch(os: PlatformOs) -> Result<PlatformArch, String> {
    #[cfg(target_os = "linux")]
    if os == PlatformOs::Linux {
        if let Some(value) = command_stdout(&["/usr/bin/uname", "/bin/uname"], "-m") {
            return PlatformArch::parse(&value);
        }
    }
    #[cfg(not(target_os = "linux"))]
    let _ = os;
    PlatformArch::parse(std::env::consts::ARCH)
}

#[derive(Debug, Default)]
struct LinuxPlatformEvidence {
    process_maps: String,
    getconf: String,
    musl_loader: bool,
    gnu_loader: bool,
    arm_hard_float_loader: bool,
    arm_soft_float_loader: bool,
}

impl LinuxPlatformEvidence {
    fn collect(arch: PlatformArch) -> Self {
        let process_maps = std::fs::read_to_string("/proc/self/maps").unwrap_or_default();
        let getconf = command_stdout(&["/usr/bin/getconf", "/bin/getconf"], "GNU_LIBC_VERSION")
            .unwrap_or_default();
        let musl_loader =
            loader_name_exists(|name| name.starts_with("ld-musl-") && name.ends_with(".so.1"));
        let gnu_loader = gnu_loader_paths(arch)
            .iter()
            .any(|path| Path::new(path).exists());
        let arm_hard_float_loader = loader_name_exists(|name| {
            name == "ld-linux-armhf.so.3" || name == "ld-musl-armhf.so.1"
        });
        let arm_soft_float_loader =
            loader_name_exists(|name| name == "ld-linux.so.3" || name == "ld-musl-arm.so.1");
        Self {
            process_maps,
            getconf,
            musl_loader,
            gnu_loader,
            arm_hard_float_loader,
            arm_soft_float_loader,
        }
    }

    fn libc(&self) -> PlatformLibc {
        let maps = self.process_maps.to_ascii_lowercase();
        if maps.contains("ld-musl-") {
            return PlatformLibc::Musl;
        }
        if maps.contains("ld-linux-") || maps.contains("libc.so.6") {
            return PlatformLibc::Gnu;
        }
        if self
            .getconf
            .trim_start()
            .to_ascii_lowercase()
            .starts_with("glibc ")
        {
            return PlatformLibc::Gnu;
        }
        match (self.musl_loader, self.gnu_loader) {
            (true, false) => PlatformLibc::Musl,
            (false, true) => PlatformLibc::Gnu,
            _ => PlatformLibc::Unknown,
        }
    }

    fn arm_abi(&self) -> PlatformAbi {
        let maps = self.process_maps.to_ascii_lowercase();
        if maps.contains("ld-linux-armhf.so.3") || maps.contains("ld-musl-armhf.so.1") {
            return PlatformAbi::Eabihf;
        }
        if maps.contains("ld-linux.so.3") || maps.contains("ld-musl-arm.so.1") {
            return PlatformAbi::Eabi;
        }
        match (self.arm_hard_float_loader, self.arm_soft_float_loader) {
            (true, false) => PlatformAbi::Eabihf,
            (false, true) => PlatformAbi::Eabi,
            _ => PlatformAbi::Unknown,
        }
    }
}

fn command_stdout(programs: &[&str], argument: &str) -> Option<String> {
    programs.iter().find_map(|program| {
        if !Path::new(program).is_file() {
            return None;
        }
        Command::new(program)
            .arg(argument)
            .output()
            .ok()
            .filter(|output| output.status.success())
            .map(|output| String::from_utf8_lossy(&output.stdout).into_owned())
    })
}

fn loader_name_exists(predicate: impl Fn(&str) -> bool) -> bool {
    ["/lib", "/lib64", "/usr/lib", "/usr/local/lib"]
        .into_iter()
        .filter_map(|directory| std::fs::read_dir(directory).ok())
        .flatten()
        .filter_map(Result::ok)
        .filter_map(|entry| entry.file_name().into_string().ok())
        .any(|name| predicate(&name))
}

fn gnu_loader_paths(arch: PlatformArch) -> &'static [&'static str] {
    match arch {
        PlatformArch::X64 => &[
            "/lib64/ld-linux-x86-64.so.2",
            "/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2",
        ],
        PlatformArch::Arm64 => &[
            "/lib/ld-linux-aarch64.so.1",
            "/lib/aarch64-linux-gnu/ld-linux-aarch64.so.1",
        ],
        PlatformArch::Armv7 => &[
            "/lib/ld-linux-armhf.so.3",
            "/lib/arm-linux-gnueabihf/ld-linux-armhf.so.3",
            "/lib/ld-linux.so.3",
        ],
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn normalizes_supported_architecture_aliases() {
        for alias in ["x86_64", "amd64", "x64"] {
            assert_eq!(PlatformArch::parse(alias).unwrap(), PlatformArch::X64);
        }
        for alias in ["aarch64", "arm64"] {
            assert_eq!(PlatformArch::parse(alias).unwrap(), PlatformArch::Arm64);
        }
        for alias in ["armv7", "armv7l", "armhf"] {
            assert_eq!(PlatformArch::parse(alias).unwrap(), PlatformArch::Armv7);
        }
        assert!(PlatformArch::parse("arm").is_err());
    }

    #[test]
    fn runtime_libc_evidence_wins_over_installed_compatibility_loaders() {
        let evidence = LinuxPlatformEvidence {
            process_maps: "/lib/ld-musl-x86_64.so.1".into(),
            musl_loader: true,
            gnu_loader: true,
            ..Default::default()
        };
        assert_eq!(evidence.libc(), PlatformLibc::Musl);

        let ambiguous = LinuxPlatformEvidence {
            musl_loader: true,
            gnu_loader: true,
            ..Default::default()
        };
        assert_eq!(ambiguous.libc(), PlatformLibc::Unknown);
    }

    #[test]
    fn builds_exact_linux_identifiers() {
        let target = PlatformTarget {
            os: PlatformOs::Linux,
            arch: PlatformArch::X64,
            libc: Some(PlatformLibc::Musl),
            abi: None,
        };
        assert_eq!(target.identifier(), "linux-x86_64-musl");
        assert_eq!(target.package_identifiers(), ["linux-x86_64-musl"]);

        let arm = PlatformTarget {
            os: PlatformOs::Linux,
            arch: PlatformArch::Armv7,
            libc: Some(PlatformLibc::Gnu),
            abi: Some(PlatformAbi::Eabihf),
        };
        assert_eq!(arm.identifier(), "linux-armv7-gnu-eabihf");

        let unknown = PlatformTarget::new(PlatformOs::Linux, PlatformArch::X64);
        assert_eq!(unknown.package_identifiers(), ["linux-x86_64-unknown"]);
    }
}
