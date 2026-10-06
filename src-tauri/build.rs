fn main() {
    preset_test_fixtures();
    tauri_build::build()
}

/// R118 · stage every committed fixture into `OUT_DIR/fixtures/` at build time.
///
/// The B-class tests need a *per-test* executable path that is upgraded in
/// place (version drift), rewritten, or deleted, while a child process may
/// still be about to exec the inode that was there before. Copying a fixture at
/// run time (the pre-R118 `stage_fixture` shape) opens a write fd on a file the
/// test then execs — the ETXTBSY window R114 measured. These tests now point a
/// symlink at one of these build-time copies and "upgrade" by repointing the
/// link (a metadata-only rename over the link): the only inode written at run
/// time is the link itself, and the bytes being exec'd were written once, by
/// cargo, before any test ran.
///
/// The presets live under the crate's `OUT_DIR`, which may sit on a different
/// filesystem than the test tempdir; a symlink repoint is immune to `EXDEV`
/// for exactly that reason.
fn preset_test_fixtures() {
    use std::path::PathBuf;

    let source = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("tests")
        .join("fixtures");
    let destination =
        PathBuf::from(std::env::var("OUT_DIR").expect("OUT_DIR is set for build scripts"))
            .join("fixtures");
    std::fs::create_dir_all(&destination).expect("create OUT_DIR/fixtures");

    // The directory listing is what `cargo` watches for additions/removals; the
    // per-file lines below catch a content edit, which does not move the
    // directory mtime.
    println!("cargo:rerun-if-changed=tests/fixtures");
    let mut entries: Vec<_> = std::fs::read_dir(&source)
        .expect("read tests/fixtures")
        .map(|entry| entry.expect("fixture directory entry"))
        .collect();
    entries.sort_by_key(|entry| entry.file_name());
    for entry in entries {
        if !entry.file_type().expect("fixture file type").is_file() {
            continue;
        }
        let name = entry.file_name();
        let preset = destination.join(&name);
        std::fs::copy(entry.path(), &preset).expect("copy fixture into OUT_DIR");
        println!(
            "cargo:rerun-if-changed=tests/fixtures/{}",
            name.to_string_lossy()
        );
        // The committed fixtures carry the execute bit; `fs::copy` preserves
        // the permission bits, and this makes the preset executable even if a
        // checkout lost them.
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            std::fs::set_permissions(&preset, std::fs::Permissions::from_mode(0o755))
                .expect("mark the OUT_DIR fixture executable");
        }
    }
}
