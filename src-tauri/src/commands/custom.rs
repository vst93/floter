use serde::{Deserialize, Serialize};
use std::process::Command;
use std::sync::Mutex;
use tauri::State;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CustomCommand {
    pub id: String,
    pub name: String,
    pub icon: Option<String>,
    pub command_type: String,
    pub command: String,
    pub shortcut: Option<String>,
    pub refresh_interval: Option<u32>,
    pub view: Option<CommandView>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CommandView {
    pub columns: Option<Vec<String>>,
    pub renderer: String,
}

pub struct CommandState {
    pub commands: Mutex<Vec<CustomCommand>>,
}

impl CommandState {
    pub fn new() -> Self {
        let commands = load_commands_from_config();
        Self {
            commands: Mutex::new(commands),
        }
    }
}

fn config_path() -> Option<std::path::PathBuf> {
    dirs::config_dir().map(|d| d.join("floter").join("commands.json"))
}

fn load_commands_from_config() -> Vec<CustomCommand> {
    if let Some(path) = config_path() {
        if path.exists() {
            if let Ok(content) = std::fs::read_to_string(path) {
                if let Ok(commands) = serde_json::from_str(&content) {
                    return commands;
                }
            }
        }
    }
    Vec::new()
}

fn save_commands_to_config(commands: &[CustomCommand]) -> Result<(), String> {
    let path = config_path().ok_or("Cannot find config directory")?;
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent).map_err(|e| e.to_string())?;
    }
    let content = serde_json::to_string_pretty(commands).map_err(|e| e.to_string())?;
    std::fs::write(path, content).map_err(|e| e.to_string())
}

#[tauri::command]
pub fn get_custom_commands(state: State<'_, CommandState>) -> Result<Vec<CustomCommand>, String> {
    let commands = state.commands.lock().map_err(|e| e.to_string())?;
    Ok(commands.clone())
}

#[tauri::command]
pub fn add_custom_command(
    state: State<'_, CommandState>,
    command: CustomCommand,
) -> Result<(), String> {
    let mut commands = state.commands.lock().map_err(|e| e.to_string())?;
    commands.push(command);
    save_commands_to_config(&commands)
}

#[tauri::command]
pub fn update_custom_command(
    state: State<'_, CommandState>,
    id: String,
    command: CustomCommand,
) -> Result<(), String> {
    let mut commands = state.commands.lock().map_err(|e| e.to_string())?;
    if let Some(pos) = commands.iter().position(|c| c.id == id) {
        commands[pos] = command;
        save_commands_to_config(&commands)
    } else {
        Err("Command not found".to_string())
    }
}

#[tauri::command]
pub fn delete_custom_command(state: State<'_, CommandState>, id: String) -> Result<(), String> {
    let mut commands = state.commands.lock().map_err(|e| e.to_string())?;
    commands.retain(|c| c.id != id);
    save_commands_to_config(&commands)
}

#[tauri::command]
pub fn execute_custom_command(command: String, _command_type: String) -> Result<String, String> {
    let output = if cfg!(target_os = "windows") {
        Command::new("cmd")
            .args(["/C", &command])
            .output()
            .map_err(|e| e.to_string())?
    } else {
        Command::new("sh")
            .args(["-c", &command])
            .output()
            .map_err(|e| e.to_string())?
    };

    let mut result = String::new();

    if !output.stdout.is_empty() {
        result.push_str(&String::from_utf8_lossy(&output.stdout));
    }

    if !output.stderr.is_empty() {
        if !result.is_empty() {
            result.push('\n');
        }
        result.push_str(&String::from_utf8_lossy(&output.stderr));
    }

    if result.is_empty() {
        result = "Command executed successfully (no output)".to_string();
    }

    Ok(result)
}
