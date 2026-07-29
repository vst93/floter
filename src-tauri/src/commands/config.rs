use serde::{Deserialize, Serialize};

/// Missing keys fall back to `Default`, so settings files written by older
/// builds keep working when new fields are introduced.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(default)]
pub struct AppSettings {
    pub hotkey: String,
    pub hide_on_blur: bool,
    pub theme: String,
    pub font_size: u32,
    pub font_family: String,
    /// Default cursor shape: "beam" | "block" | "underline".
    pub cursor_shape: String,
    /// UI language: "en" | "zh".
    pub language: String,
}

impl Default for AppSettings {
    fn default() -> Self {
        Self {
            hotkey: "Ctrl+Shift+Space".to_string(),
            hide_on_blur: true,
            theme: "dark".to_string(),
            font_size: 14,
            font_family: "monospace".to_string(),
            cursor_shape: "beam".to_string(),
            language: "en".to_string(),
        }
    }
}

/// Load settings from disk, falling back to defaults when missing or invalid.
pub fn load_settings() -> AppSettings {
    let Some(config_path) = dirs::config_dir().map(|d| d.join("floter").join("settings.json"))
    else {
        return AppSettings::default();
    };
    if let Ok(content) = std::fs::read_to_string(&config_path) {
        if let Ok(settings) = serde_json::from_str::<AppSettings>(&content) {
            return settings;
        }
    }
    AppSettings::default()
}

#[tauri::command]
pub fn get_settings() -> Result<AppSettings, String> {
    Ok(load_settings())
}

#[tauri::command]
pub fn save_settings(app: tauri::AppHandle, settings: AppSettings) -> Result<(), String> {
    let config_dir = dirs::config_dir().ok_or("Cannot find config directory")?;
    let floter_dir = config_dir.join("floter");
    std::fs::create_dir_all(&floter_dir).map_err(|e| e.to_string())?;
    let config_path = floter_dir.join("settings.json");
    let content = serde_json::to_string_pretty(&settings).map_err(|e| e.to_string())?;
    std::fs::write(config_path, content).map_err(|e| e.to_string())?;
    crate::apply_tray_language(&app, &settings.language);
    Ok(())
}
