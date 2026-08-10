use crate::extensions::catalog::invocation_from_entry;
use crate::extensions::lock::{ExtensionLockEntry, ExtensionsLock};
use crate::extensions::provider::ProviderInvocation;
use crate::extensions::provider::{
    execution_plan, CommandDescriptor, ExecutionDescriptor, ExecutionMode, ExecutionPlan,
    WorkingDirectory,
};
use crate::extensions::ExtensionState;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::{BTreeMap, HashSet};
use std::io::Write;
use std::path::Path;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ConfigurationDescriptor {
    pub owner: ConfigurationOwner,
    #[serde(default)]
    pub open_command: Vec<String>,
    #[serde(default)]
    pub schema: Vec<ConfigurationField>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ConfigurationOwner {
    Host,
    Tool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ConfigurationField {
    pub key: String,
    #[serde(rename = "type")]
    pub field_type: ConfigurationFieldType,
    #[serde(default)]
    pub label: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub required: bool,
    pub default: Option<Value>,
    #[serde(default)]
    pub options: Vec<Value>,
    pub minimum: Option<f64>,
    pub maximum: Option<f64>,
    pub environment: Option<String>,
    pub argument: Option<String>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum ConfigurationFieldType {
    Text,
    Password,
    Path,
    Select,
    MultiSelect,
    Boolean,
    Number,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ExtensionConfiguration {
    pub descriptor: ConfigurationDescriptor,
    pub values: BTreeMap<String, Value>,
    pub open_plan: Option<ExecutionPlan>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StoredConfiguration {
    values: BTreeMap<String, Value>,
    schema: Vec<ConfigurationField>,
}

#[derive(Deserialize)]
#[serde(untagged)]
enum StoredConfigurationFormat {
    Current(StoredConfiguration),
    Legacy(BTreeMap<String, Value>),
}

#[derive(Deserialize)]
struct ConfigurationEnvelope {
    configuration: ConfigurationDescriptor,
}

pub async fn get(
    state: &ExtensionState,
    extension_id: &str,
) -> Result<ExtensionConfiguration, String> {
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let entry = lock.get(extension_id)?;
    let (descriptor, invocation) = descriptor(state, entry).await?;
    validate_descriptor(&descriptor)?;
    let values = if descriptor.owner == ConfigurationOwner::Host {
        load_values(&state.paths.data, extension_id)?
    } else {
        BTreeMap::new()
    };
    let open_plan = tool_configuration_plan(&descriptor, &invocation)?;
    Ok(ExtensionConfiguration {
        descriptor,
        values,
        open_plan,
    })
}

pub async fn set(
    state: &ExtensionState,
    extension_id: &str,
    values: BTreeMap<String, Value>,
) -> Result<ExtensionConfiguration, String> {
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let entry = lock.get(extension_id)?;
    let (descriptor, _) = descriptor(state, entry).await?;
    validate_descriptor(&descriptor)?;
    if descriptor.owner != ConfigurationOwner::Host {
        return Err(format!(
            "Extension {extension_id} manages its own configuration"
        ));
    }
    validate_values(&descriptor.schema, &values)?;
    save_values(&state.paths.data, extension_id, &descriptor.schema, &values)?;
    Ok(ExtensionConfiguration {
        descriptor,
        values,
        open_plan: None,
    })
}

pub fn apply_persisted_configuration(
    data_root: &Path,
    invocation: &mut ProviderInvocation,
) -> Result<Vec<String>, String> {
    let stored = load_stored(data_root, &invocation.extension_id)?;
    let mut args = Vec::new();
    for field in &stored.schema {
        let value = stored.values.get(&field.key);
        let (Some(environment), Some(value)) = (&field.environment, value) else {
            if let (Some(argument), Some(value)) = (&field.argument, value) {
                append_argument(&mut args, argument, value);
            }
            continue;
        };
        if let Some(rendered) = render_value(value) {
            invocation
                .config
                .environment
                .insert(environment.clone(), rendered);
        }
        if let Some(argument) = &field.argument {
            append_argument(&mut args, argument, value);
        }
    }
    Ok(args)
}

fn append_argument(args: &mut Vec<String>, argument: &str, value: &Value) {
    if value.as_bool() == Some(false) || value.is_null() {
        return;
    }
    args.push(argument.to_string());
    if value.as_bool() != Some(true) {
        if let Some(value) = render_value(value) {
            args.push(value);
        }
    }
}

fn render_value(value: &Value) -> Option<String> {
    match value {
        Value::String(value) => Some(value.clone()),
        Value::Bool(value) => Some(value.to_string()),
        Value::Number(value) => Some(value.to_string()),
        Value::Array(values) => Some(
            values
                .iter()
                .filter_map(Value::as_str)
                .collect::<Vec<_>>()
                .join(","),
        ),
        _ => None,
    }
}

async fn descriptor(
    state: &ExtensionState,
    entry: &ExtensionLockEntry,
) -> Result<(ConfigurationDescriptor, ProviderInvocation), String> {
    let mut invocation = invocation_from_entry(entry)?;
    let _ = apply_persisted_configuration(&state.paths.data, &mut invocation)?;
    let envelope: ConfigurationEnvelope = state.provider.call_config(&invocation).await?;
    Ok((envelope.configuration, invocation))
}

fn tool_configuration_plan(
    descriptor: &ConfigurationDescriptor,
    invocation: &ProviderInvocation,
) -> Result<Option<ExecutionPlan>, String> {
    if descriptor.owner != ConfigurationOwner::Tool {
        return Ok(None);
    }
    let command = CommandDescriptor {
        id: "configuration".into(),
        name: "Configuration".into(),
        description: String::new(),
        aliases: Vec::new(),
        keywords: Vec::new(),
        execution: ExecutionDescriptor {
            program: "self".into(),
            args_prefix: descriptor.open_command.clone(),
            mode: ExecutionMode::Pty,
            working_directory: WorkingDirectory::Current,
        },
        arguments: Vec::new(),
    };
    execution_plan(&command, invocation, Vec::new(), None).map(Some)
}

fn validate_descriptor(descriptor: &ConfigurationDescriptor) -> Result<(), String> {
    match descriptor.owner {
        ConfigurationOwner::Tool => {
            if descriptor.open_command.is_empty() {
                return Err("Tool-managed configuration must provide openCommand".to_string());
            }
            if !descriptor.schema.is_empty() {
                return Err("Tool-managed configuration cannot provide a host schema".to_string());
            }
        }
        ConfigurationOwner::Host => {
            if descriptor.schema.is_empty() {
                return Err("Host-managed configuration must provide a schema".to_string());
            }
            let mut keys = HashSet::new();
            for field in &descriptor.schema {
                if field.key.is_empty()
                    || !field.key.bytes().all(|byte| {
                        byte.is_ascii_alphanumeric() || matches!(byte, b'.' | b'_' | b'-')
                    })
                    || !keys.insert(&field.key)
                {
                    return Err(format!(
                        "Invalid or duplicate configuration key: {}",
                        field.key
                    ));
                }
                if field.field_type == ConfigurationFieldType::Select && field.options.is_empty() {
                    return Err(format!("Select field {} has no options", field.key));
                }
            }
        }
    }
    Ok(())
}

fn validate_values(
    schema: &[ConfigurationField],
    values: &BTreeMap<String, Value>,
) -> Result<(), String> {
    let fields = schema
        .iter()
        .map(|field| (field.key.as_str(), field))
        .collect::<BTreeMap<_, _>>();
    for key in values.keys() {
        if !fields.contains_key(key.as_str()) {
            return Err(format!("Unknown configuration key: {key}"));
        }
    }
    for field in schema {
        let Some(value) = values.get(&field.key).or(field.default.as_ref()) else {
            if field.required {
                return Err(format!("Missing required configuration key: {}", field.key));
            }
            continue;
        };
        let type_matches = match field.field_type {
            ConfigurationFieldType::Text
            | ConfigurationFieldType::Password
            | ConfigurationFieldType::Path
            | ConfigurationFieldType::Select => value.is_string(),
            ConfigurationFieldType::MultiSelect => value
                .as_array()
                .is_some_and(|values| values.iter().all(Value::is_string)),
            ConfigurationFieldType::Boolean => value.is_boolean(),
            ConfigurationFieldType::Number => value.is_number(),
        };
        if !type_matches {
            return Err(format!("Invalid value type for {}", field.key));
        }
        if matches!(
            field.field_type,
            ConfigurationFieldType::Select | ConfigurationFieldType::MultiSelect
        ) {
            let selected = value
                .as_array()
                .map_or_else(|| vec![value], |values| values.iter().collect());
            if selected.iter().any(|value| !field.options.contains(value)) {
                return Err(format!("Value for {} is not an allowed option", field.key));
            }
        }
        if let Some(number) = value.as_f64() {
            if field.minimum.is_some_and(|minimum| number < minimum)
                || field.maximum.is_some_and(|maximum| number > maximum)
            {
                return Err(format!(
                    "Value for {} is outside its allowed range",
                    field.key
                ));
            }
        }
    }
    Ok(())
}

fn values_path(data_root: &Path, extension_id: &str) -> Result<std::path::PathBuf, String> {
    crate::extensions::lock::validate_id(extension_id)?;
    Ok(data_root.join(extension_id).join("config.json"))
}

fn load_values(data_root: &Path, extension_id: &str) -> Result<BTreeMap<String, Value>, String> {
    Ok(load_stored(data_root, extension_id)?.values)
}

fn load_stored(data_root: &Path, extension_id: &str) -> Result<StoredConfiguration, String> {
    let path = values_path(data_root, extension_id)?;
    if !path.exists() {
        return Ok(StoredConfiguration {
            values: BTreeMap::new(),
            schema: Vec::new(),
        });
    }
    let stored: StoredConfigurationFormat = serde_json::from_slice(
        &std::fs::read(&path)
            .map_err(|error| format!("Cannot read configuration {}: {error}", path.display()))?,
    )
    .map_err(|error| format!("Invalid configuration {}: {error}", path.display()))?;
    Ok(match stored {
        StoredConfigurationFormat::Current(stored) => stored,
        StoredConfigurationFormat::Legacy(values) => StoredConfiguration {
            values,
            schema: Vec::new(),
        },
    })
}

fn save_values(
    data_root: &Path,
    extension_id: &str,
    schema: &[ConfigurationField],
    values: &BTreeMap<String, Value>,
) -> Result<(), String> {
    let path = values_path(data_root, extension_id)?;
    let parent = path
        .parent()
        .ok_or("Invalid extension configuration path")?;
    std::fs::create_dir_all(parent)
        .map_err(|error| format!("Cannot create extension data directory: {error}"))?;
    let bytes = serde_json::to_vec_pretty(&StoredConfiguration {
        values: values.clone(),
        schema: schema.to_vec(),
    })
    .map_err(|error| format!("Cannot serialize extension configuration: {error}"))?;
    let mut temporary = tempfile::NamedTempFile::new_in(parent)
        .map_err(|error| format!("Cannot create configuration temporary file: {error}"))?;
    temporary
        .write_all(&bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write extension configuration: {error}"))?;
    temporary
        .persist(path)
        .map(|_| ())
        .map_err(|error| format!("Cannot persist extension configuration: {error}"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rejects_unknown_configuration_values() {
        let values = BTreeMap::from([("other".to_string(), Value::Bool(true))]);
        assert!(validate_values(&[], &values).is_err());
    }

    #[test]
    fn validates_select_options() {
        let field = ConfigurationField {
            key: "language".into(),
            field_type: ConfigurationFieldType::Select,
            label: String::new(),
            description: String::new(),
            required: true,
            default: None,
            options: vec![Value::String("en".into()), Value::String("zh".into())],
            minimum: None,
            maximum: None,
            environment: None,
            argument: None,
        };
        let values = BTreeMap::from([("language".into(), Value::String("fr".into()))]);
        assert!(validate_values(&[field], &values).is_err());
    }
}
