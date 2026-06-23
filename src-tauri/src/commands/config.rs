use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AppSettings {
    pub hotkey: String,
    pub hide_on_blur: bool,
    pub theme: String,
    pub font_size: u32,
    pub font_family: String,
}

impl Default for AppSettings {
    fn default() -> Self {
        Self {
            hotkey: "Ctrl+Shift+Space".to_string(),
            hide_on_blur: true,
            theme: "dark".to_string(),
            font_size: 14,
            font_family: "monospace".to_string(),
        }
    }
}

#[tauri::command]
pub fn get_settings() -> Result<AppSettings, String> {
    let config_path = dirs::config_dir()
        .ok_or("Cannot find config directory")?
        .join("floter")
        .join("settings.json");

    if config_path.exists() {
        let content = std::fs::read_to_string(&config_path).map_err(|e| e.to_string())?;
        serde_json::from_str(&content).map_err(|e| e.to_string())
    } else {
        Ok(AppSettings::default())
    }
}

#[tauri::command]
pub fn save_settings(settings: AppSettings) -> Result<(), String> {
    let config_dir = dirs::config_dir().ok_or("Cannot find config directory")?;
    let floter_dir = config_dir.join("floter");
    std::fs::create_dir_all(&floter_dir).map_err(|e| e.to_string())?;
    let config_path = floter_dir.join("settings.json");
    let content = serde_json::to_string_pretty(&settings).map_err(|e| e.to_string())?;
    std::fs::write(config_path, content).map_err(|e| e.to_string())
}
