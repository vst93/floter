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
use std::path::{Path, PathBuf};

const PASSWORD_PLACEHOLDER: &str = "********";
const STORED_PASSWORD_PLACEHOLDER: &str = "[REDACTED]";

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct ConfigurationDescriptor {
    #[serde(default = "default_config_version")]
    pub config_version: u64,
    pub owner: ConfigurationOwner,
    #[serde(default)]
    pub open_command: Vec<String>,
    #[serde(default)]
    pub schema: Vec<ConfigurationField>,
    #[serde(default)]
    pub environment_mapping: BTreeMap<String, String>,
}

fn default_config_version() -> u64 {
    1
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ConfigurationOwner {
    Host,
    Tool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
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
    pub min_length: Option<usize>,
    pub max_length: Option<usize>,
    #[serde(alias = "environment")]
    pub env_var: Option<String>,
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

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct ConfigurationExportDocument {
    format_version: u32,
    extension_id: String,
    config_version: u64,
    values: BTreeMap<String, Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StoredConfiguration {
    #[serde(default)]
    config_version: u64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    secret_generation: Option<String>,
    values: BTreeMap<String, Value>,
    schema: Vec<ConfigurationField>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StoredSecrets {
    generation: String,
    values: BTreeMap<String, Value>,
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
    let _guard = state.mutation_lock.lock().await;
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let entry = lock.get(extension_id)?;
    let (descriptor, invocation) = descriptor(state, entry).await?;
    validate_descriptor(&descriptor)?;
    let values = if descriptor.owner == ConfigurationOwner::Host {
        let stored = load_stored(&state.paths.data, extension_id)?;
        let (values, migrated) = migrate_values(&stored, &descriptor)?;
        if migrated {
            save_values(
                &state.paths.data,
                extension_id,
                descriptor.config_version,
                &descriptor.schema,
                &values,
            )?;
        }
        values
    } else {
        BTreeMap::new()
    };
    let open_plan = tool_configuration_plan(&descriptor, &invocation)?
        .map(|plan| state.protect_execution_plan(plan))
        .transpose()?;
    Ok(configuration_for_ipc(descriptor, values, open_plan))
}

pub async fn set(
    state: &ExtensionState,
    extension_id: &str,
    values: BTreeMap<String, Value>,
) -> Result<ExtensionConfiguration, String> {
    let _guard = state.mutation_lock.lock().await;
    set_locked(state, extension_id, values).await
}

async fn set_locked(
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
    let stored = load_stored(&state.paths.data, extension_id)?;
    let (current, _) = migrate_values(&stored, &descriptor)?;
    let values = preserve_password_placeholders(&descriptor.schema, current, values);
    validate_values(&descriptor.schema, &values)?;
    let values = materialize_defaults(&descriptor.schema, values);
    save_values(
        &state.paths.data,
        extension_id,
        descriptor.config_version,
        &descriptor.schema,
        &values,
    )?;
    Ok(configuration_for_ipc(descriptor, values, None))
}

pub async fn export_json(
    state: &ExtensionState,
    extension_id: &str,
    values: BTreeMap<String, Value>,
) -> Result<String, String> {
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
    let document = ConfigurationExportDocument {
        format_version: 1,
        extension_id: extension_id.to_string(),
        config_version: descriptor.config_version,
        values: redact_exported_passwords(&descriptor.schema, values),
    };
    let mut json = serde_json::to_string_pretty(&document)
        .map_err(|error| format!("Cannot serialize extension configuration export: {error}"))?;
    json.push('\n');
    Ok(json)
}

pub fn write_export(path: &Path, json: &str) -> Result<(), String> {
    atomic_write(path, json.as_bytes())
}

pub(crate) fn export_values(
    data_root: &Path,
    extension_id: &str,
) -> Result<BTreeMap<String, Value>, String> {
    let stored = load_stored(data_root, extension_id)?;
    Ok(redact_exported_passwords(&stored.schema, stored.values))
}

pub(crate) async fn preflight_import_values(
    state: &ExtensionState,
    extension_id: &str,
    imported: &BTreeMap<String, Value>,
) -> Result<(), String> {
    if imported.is_empty() {
        return Ok(());
    }
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let entry = lock.get(extension_id)?;
    let (descriptor, _) = descriptor(state, entry).await?;
    validate_descriptor(&descriptor)?;
    if descriptor.owner != ConfigurationOwner::Host {
        return Err(format!(
            "Extension {extension_id} manages its own configuration"
        ));
    }
    let stored = load_stored(&state.paths.data, extension_id)?;
    let (current, _) = migrate_values(&stored, &descriptor)?;
    let _ = merge_imported_values(&descriptor.schema, current, imported)?;
    Ok(())
}

pub(crate) async fn import_values_locked(
    state: &ExtensionState,
    extension_id: &str,
    imported: &BTreeMap<String, Value>,
) -> Result<bool, String> {
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let entry = lock.get(extension_id)?;
    let (descriptor, _) = descriptor(state, entry).await?;
    validate_descriptor(&descriptor)?;
    if descriptor.owner != ConfigurationOwner::Host {
        return imported
            .is_empty()
            .then_some(false)
            .ok_or_else(|| format!("Extension {extension_id} manages its own configuration"));
    }

    let stored = load_stored(&state.paths.data, extension_id)?;
    let (current, _) = migrate_values(&stored, &descriptor)?;
    let (values, changed) = merge_imported_values(&descriptor.schema, current, imported)?;
    if changed {
        save_values(
            &state.paths.data,
            extension_id,
            descriptor.config_version,
            &descriptor.schema,
            &values,
        )?;
    }
    Ok(changed)
}

fn redact_exported_passwords(
    schema: &[ConfigurationField],
    mut values: BTreeMap<String, Value>,
) -> BTreeMap<String, Value> {
    for field in schema {
        if field.field_type == ConfigurationFieldType::Password && values.contains_key(&field.key) {
            values.insert(
                field.key.clone(),
                Value::String(PASSWORD_PLACEHOLDER.to_string()),
            );
        }
    }
    values
}

fn merge_imported_values(
    schema: &[ConfigurationField],
    mut current: BTreeMap<String, Value>,
    imported: &BTreeMap<String, Value>,
) -> Result<(BTreeMap<String, Value>, bool), String> {
    let fields = schema
        .iter()
        .map(|field| (field.key.as_str(), field))
        .collect::<BTreeMap<_, _>>();
    let mut changed = false;
    for (key, value) in imported {
        let field = fields
            .get(key.as_str())
            .ok_or_else(|| format!("Unknown configuration key: {key}"))?;
        if field.field_type == ConfigurationFieldType::Password
            && value.as_str() == Some(PASSWORD_PLACEHOLDER)
        {
            continue;
        }
        validate_field_value(field, value)?;
        if current.get(key) != Some(value) {
            current.insert(key.clone(), value.clone());
            changed = true;
        }
    }
    Ok((current, changed))
}

fn materialize_defaults(
    schema: &[ConfigurationField],
    mut values: BTreeMap<String, Value>,
) -> BTreeMap<String, Value> {
    for field in schema {
        if let Some(default) = &field.default {
            values
                .entry(field.key.clone())
                .or_insert_with(|| default.clone());
        }
    }
    values
}

fn preserve_password_placeholders(
    schema: &[ConfigurationField],
    current: BTreeMap<String, Value>,
    mut submitted: BTreeMap<String, Value>,
) -> BTreeMap<String, Value> {
    for field in schema {
        if field.field_type != ConfigurationFieldType::Password
            || submitted.get(&field.key).and_then(Value::as_str) != Some(PASSWORD_PLACEHOLDER)
        {
            continue;
        }
        if let Some(value) = current.get(&field.key) {
            submitted.insert(field.key.clone(), value.clone());
        } else {
            submitted.remove(&field.key);
        }
    }
    submitted
}

fn configuration_for_ipc(
    mut descriptor: ConfigurationDescriptor,
    values: BTreeMap<String, Value>,
    open_plan: Option<ExecutionPlan>,
) -> ExtensionConfiguration {
    descriptor.schema = redact_schema_for_ipc(&descriptor.schema);
    ExtensionConfiguration {
        values: redact_exported_passwords(&descriptor.schema, values),
        descriptor,
        open_plan,
    }
}

fn redact_schema_for_ipc(schema: &[ConfigurationField]) -> Vec<ConfigurationField> {
    let mut schema = schema.to_vec();
    for field in &mut schema {
        if field.field_type == ConfigurationFieldType::Password && field.default.is_some() {
            field.default = Some(Value::String(PASSWORD_PLACEHOLDER.to_string()));
        }
    }
    schema
}

pub fn apply_persisted_configuration(
    data_root: &Path,
    invocation: &mut ProviderInvocation,
) -> Result<Vec<String>, String> {
    let stored = load_stored(data_root, &invocation.extension_id)?;
    let mut args = Vec::new();
    for field in &stored.schema {
        let value = stored.values.get(&field.key);
        let (Some(environment), Some(value)) = (&field.env_var, value) else {
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
    let invocation = crate::extensions::registry::provider_invocation(entry)?;
    let mut configured_invocation = invocation.clone();
    let _ = apply_persisted_configuration(&state.paths.data, &mut configured_invocation)?;
    let mut envelope: ConfigurationEnvelope =
        state.provider.call_config(&configured_invocation).await?;
    apply_environment_mapping(&mut envelope.configuration)?;
    Ok((envelope.configuration, invocation))
}

fn apply_environment_mapping(descriptor: &mut ConfigurationDescriptor) -> Result<(), String> {
    for (key, env_var) in &descriptor.environment_mapping {
        let field = descriptor
            .schema
            .iter_mut()
            .find(|field| &field.key == key)
            .ok_or_else(|| format!("environmentMapping references unknown key: {key}"))?;
        if field.env_var.is_none() {
            field.env_var = Some(env_var.clone());
        }
    }
    Ok(())
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

pub fn validate_descriptor(descriptor: &ConfigurationDescriptor) -> Result<(), String> {
    if descriptor.config_version == 0 {
        return Err("configVersion must be greater than zero".to_string());
    }
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
                if matches!(
                    field.field_type,
                    ConfigurationFieldType::Select | ConfigurationFieldType::MultiSelect
                ) && field.options.is_empty()
                {
                    return Err(format!("Selection field {} has no options", field.key));
                }
                if field
                    .env_var
                    .as_deref()
                    .is_some_and(|name| !valid_env_var(name))
                {
                    return Err(format!("Invalid envVar for {}", field.key));
                }
                if field
                    .minimum
                    .zip(field.maximum)
                    .is_some_and(|(min, max)| min > max)
                {
                    return Err(format!("Invalid numeric range for {}", field.key));
                }
                if field
                    .min_length
                    .zip(field.max_length)
                    .is_some_and(|(min, max)| min > max)
                {
                    return Err(format!("Invalid text length range for {}", field.key));
                }
                if let Some(default) = &field.default {
                    validate_field_value(field, default)?;
                }
            }
        }
    }
    Ok(())
}

fn valid_env_var(name: &str) -> bool {
    let mut bytes = name.bytes();
    bytes
        .next()
        .is_some_and(|byte| byte.is_ascii_alphabetic() || byte == b'_')
        && bytes.all(|byte| byte.is_ascii_alphanumeric() || byte == b'_')
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
        validate_field_value(field, value)?;
    }
    Ok(())
}

fn validate_field_value(field: &ConfigurationField, value: &Value) -> Result<(), String> {
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
    if field.field_type == ConfigurationFieldType::Text {
        if let Some(text) = value.as_str() {
            let length = text.chars().count();
            if field.min_length.is_some_and(|minimum| length < minimum)
                || field.max_length.is_some_and(|maximum| length > maximum)
            {
                return Err(format!(
                    "Value for {} is outside its allowed length",
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

fn secrets_path(data_root: &Path, extension_id: &str) -> Result<std::path::PathBuf, String> {
    crate::extensions::lock::validate_id(extension_id)?;
    Ok(data_root.join(extension_id).join("config.secrets.json"))
}

fn secrets_generation_dir(data_root: &Path, extension_id: &str) -> Result<PathBuf, String> {
    crate::extensions::lock::validate_id(extension_id)?;
    Ok(data_root.join(extension_id).join("config-secrets"))
}

fn secrets_generation_path(
    data_root: &Path,
    extension_id: &str,
    generation: &str,
) -> Result<PathBuf, String> {
    if generation.is_empty()
        || !generation
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || byte == b'-')
    {
        return Err("Invalid configuration secret generation".to_string());
    }
    Ok(secrets_generation_dir(data_root, extension_id)?.join(format!("{generation}.json")))
}

fn load_stored(data_root: &Path, extension_id: &str) -> Result<StoredConfiguration, String> {
    let path = values_path(data_root, extension_id)?;
    if !path.exists() {
        return Ok(StoredConfiguration {
            config_version: 0,
            secret_generation: None,
            values: BTreeMap::new(),
            schema: Vec::new(),
        });
    }
    let stored: StoredConfigurationFormat = serde_json::from_slice(
        &std::fs::read(&path)
            .map_err(|error| format!("Cannot read configuration {}: {error}", path.display()))?,
    )
    .map_err(|error| format!("Invalid configuration {}: {error}", path.display()))?;
    let mut stored = match stored {
        StoredConfigurationFormat::Current(stored) => stored,
        StoredConfigurationFormat::Legacy(values) => StoredConfiguration {
            config_version: 0,
            secret_generation: None,
            values,
            schema: Vec::new(),
        },
    };
    let secrets = match stored.secret_generation.as_deref() {
        Some(generation) => load_secret_generation(data_root, extension_id, generation)?,
        None => load_legacy_secrets(data_root, extension_id)?,
    };
    if let Some(secrets) = secrets {
        stored.values.extend(secrets);
    } else {
        for field in &stored.schema {
            if field.field_type == ConfigurationFieldType::Password
                && stored.values.get(&field.key).and_then(Value::as_str)
                    == Some(STORED_PASSWORD_PLACEHOLDER)
            {
                stored.values.remove(&field.key);
            }
        }
    }
    Ok(stored)
}

fn load_secret_generation(
    data_root: &Path,
    extension_id: &str,
    generation: &str,
) -> Result<Option<BTreeMap<String, Value>>, String> {
    let Ok(path) = secrets_generation_path(data_root, extension_id, generation) else {
        return Ok(None);
    };
    if !path.exists() {
        return Ok(None);
    }
    let secrets: StoredSecrets =
        match serde_json::from_slice(&std::fs::read(&path).map_err(|error| {
            format!(
                "Cannot read configuration secrets {}: {error}",
                path.display()
            )
        })?) {
            Ok(secrets) => secrets,
            Err(_) => return Ok(None),
        };
    if secrets.generation != generation {
        return Ok(None);
    }
    Ok(Some(secrets.values))
}

fn load_legacy_secrets(
    data_root: &Path,
    extension_id: &str,
) -> Result<Option<BTreeMap<String, Value>>, String> {
    let path = secrets_path(data_root, extension_id)?;
    if !path.exists() {
        return Ok(None);
    }
    serde_json::from_slice(&std::fs::read(&path).map_err(|error| {
        format!(
            "Cannot read configuration secrets {}: {error}",
            path.display()
        )
    })?)
    .map(Some)
    .map_err(|error| format!("Invalid configuration secrets {}: {error}", path.display()))
}

fn migrate_values(
    stored: &StoredConfiguration,
    descriptor: &ConfigurationDescriptor,
) -> Result<(BTreeMap<String, Value>, bool), String> {
    let previous_fields = stored
        .schema
        .iter()
        .map(|field| (field.key.as_str(), field))
        .collect::<BTreeMap<_, _>>();
    let mut values = BTreeMap::new();
    for field in &descriptor.schema {
        let compatible = previous_fields
            .get(field.key.as_str())
            .is_some_and(|previous| previous.field_type == field.field_type)
            || stored.schema.is_empty();
        if compatible {
            if let Some(value) = stored.values.get(&field.key) {
                if validate_field_value(field, value).is_ok() {
                    values.insert(field.key.clone(), value.clone());
                    continue;
                }
            }
        }
        if let Some(default) = &field.default {
            validate_field_value(field, default)?;
            values.insert(field.key.clone(), default.clone());
        }
    }
    let migrated = stored.config_version != descriptor.config_version
        || stored.schema != redacted_schema(&descriptor.schema)
        || stored.values != values;
    Ok((values, migrated))
}

fn save_values(
    data_root: &Path,
    extension_id: &str,
    config_version: u64,
    schema: &[ConfigurationField],
    values: &BTreeMap<String, Value>,
) -> Result<(), String> {
    save_values_with_failure(
        data_root,
        extension_id,
        config_version,
        schema,
        values,
        None,
    )
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum SaveFailure {
    ProtectSecrets,
    CommitPublic,
}

fn save_values_with_failure(
    data_root: &Path,
    extension_id: &str,
    config_version: u64,
    schema: &[ConfigurationField],
    values: &BTreeMap<String, Value>,
    failure: Option<SaveFailure>,
) -> Result<(), String> {
    let path = values_path(data_root, extension_id)?;
    let parent = path
        .parent()
        .ok_or("Invalid extension configuration path")?;
    std::fs::create_dir_all(parent)
        .map_err(|error| format!("Cannot create extension data directory: {error}"))?;
    let password_keys = schema
        .iter()
        .filter(|field| field.field_type == ConfigurationFieldType::Password)
        .map(|field| field.key.as_str())
        .collect::<HashSet<_>>();
    let mut public_values = values.clone();
    let mut secrets = BTreeMap::new();
    for key in password_keys {
        if let Some(value) = public_values.get_mut(key) {
            secrets.insert(key.to_string(), value.clone());
            *value = Value::String(STORED_PASSWORD_PLACEHOLDER.to_string());
        }
    }
    let generation = uuid::Uuid::new_v4().to_string();
    let secret_document = StoredSecrets {
        generation: generation.clone(),
        values: secrets,
    };
    let secret_bytes = serde_json::to_vec_pretty(&secret_document)
        .map_err(|error| format!("Cannot serialize extension configuration secrets: {error}"))?;
    let secret_dir = secrets_generation_dir(data_root, extension_id)?;
    std::fs::create_dir_all(&secret_dir)
        .map_err(|error| format!("Cannot create configuration secret directory: {error}"))?;
    let secret_path = secrets_generation_path(data_root, extension_id, &generation)?;
    let temporary = write_temporary(&secret_dir, &secret_bytes)?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        if failure == Some(SaveFailure::ProtectSecrets) {
            return Err("Injected configuration secret chmod failure".to_string());
        }
        temporary
            .as_file()
            .set_permissions(std::fs::Permissions::from_mode(0o600))
            .map_err(|error| format!("Cannot protect configuration secrets: {error}"))?;
        temporary
            .as_file()
            .sync_all()
            .map_err(|error| format!("Cannot sync protected configuration secrets: {error}"))?;
    }
    temporary
        .persist(&secret_path)
        .map_err(|error| format!("Cannot persist extension configuration secrets: {error}"))?;
    crate::extensions::lock::sync_directory(&secret_dir)
        .map_err(|error| format!("Cannot sync configuration secret directory: {error}"))?;

    if failure == Some(SaveFailure::CommitPublic) {
        return Err("Injected configuration public commit failure".to_string());
    }
    let stored_schema = redacted_schema(schema);
    let bytes = serde_json::to_vec_pretty(&StoredConfiguration {
        config_version,
        secret_generation: Some(generation.clone()),
        values: public_values,
        schema: stored_schema,
    })
    .map_err(|error| format!("Cannot serialize extension configuration: {error}"))?;
    atomic_write(&path, &bytes)?;
    Ok(())
}

fn cleanup_secret_generations(directory: &Path, current: &str) {
    let Ok(entries) = std::fs::read_dir(directory) else {
        return;
    };
    for entry in entries.flatten() {
        if entry.file_name().to_str() != Some(&format!("{current}.json")) {
            let _ = std::fs::remove_file(entry.path());
        }
    }
}

pub(crate) fn recover_configurations(data_root: &Path) -> Result<(), String> {
    let entries = std::fs::read_dir(data_root)
        .map_err(|error| format!("Cannot scan extension configuration data: {error}"))?;
    for entry in entries {
        let entry = entry.map_err(|error| format!("Cannot scan extension data entry: {error}"))?;
        if !entry.path().is_dir() {
            continue;
        }
        let Some(extension_id) = entry.file_name().to_str().map(str::to_string) else {
            continue;
        };
        if crate::extensions::lock::validate_id(&extension_id).is_err() {
            continue;
        }
        recover_configuration(data_root, &extension_id)?;
    }
    Ok(())
}

fn recover_configuration(data_root: &Path, extension_id: &str) -> Result<(), String> {
    let path = values_path(data_root, extension_id)?;
    if !path.exists() {
        return Ok(());
    }
    let bytes = std::fs::read(&path)
        .map_err(|error| format!("Cannot read configuration {}: {error}", path.display()))?;
    let StoredConfigurationFormat::Current(mut stored) = serde_json::from_slice(&bytes)
        .map_err(|error| format!("Invalid configuration {}: {error}", path.display()))?
    else {
        return Ok(());
    };
    let Some(generation) = stored.secret_generation.clone() else {
        return Ok(());
    };
    if load_secret_generation(data_root, extension_id, &generation)?.is_some() {
        cleanup_secret_generations(
            &secrets_generation_dir(data_root, extension_id)?,
            &generation,
        );
        let legacy = secrets_path(data_root, extension_id)?;
        if legacy.exists() {
            let _ = std::fs::remove_file(legacy);
        }
        return Ok(());
    }
    stored.secret_generation = None;
    for field in &stored.schema {
        if field.field_type == ConfigurationFieldType::Password {
            stored.values.remove(&field.key);
        }
    }
    let repaired = serde_json::to_vec_pretty(&stored)
        .map_err(|error| format!("Cannot serialize repaired configuration: {error}"))?;
    atomic_write(&path, &repaired)
}

fn redacted_schema(schema: &[ConfigurationField]) -> Vec<ConfigurationField> {
    let mut schema = schema.to_vec();
    for field in &mut schema {
        if field.field_type == ConfigurationFieldType::Password && field.default.is_some() {
            field.default = Some(Value::String(STORED_PASSWORD_PLACEHOLDER.to_string()));
        }
    }
    schema
}

fn atomic_write(path: &Path, bytes: &[u8]) -> Result<(), String> {
    let parent = path
        .parent()
        .ok_or("Invalid extension configuration path")?;
    let temporary = write_temporary(parent, bytes)?;
    temporary
        .persist(path)
        .map(|_| ())
        .map_err(|error| format!("Cannot persist extension configuration: {error}"))?;
    crate::extensions::lock::sync_directory(parent)
        .map_err(|error| format!("Cannot sync extension configuration directory: {error}"))
}

fn write_temporary(parent: &Path, bytes: &[u8]) -> Result<tempfile::NamedTempFile, String> {
    let mut temporary = tempfile::NamedTempFile::new_in(parent)
        .map_err(|error| format!("Cannot create configuration temporary file: {error}"))?;
    temporary
        .write_all(bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write extension configuration: {error}"))?;
    Ok(temporary)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn field(key: &str, field_type: ConfigurationFieldType) -> ConfigurationField {
        ConfigurationField {
            key: key.into(),
            field_type,
            label: String::new(),
            description: String::new(),
            required: false,
            default: None,
            options: Vec::new(),
            minimum: None,
            maximum: None,
            min_length: None,
            max_length: None,
            env_var: None,
            argument: None,
        }
    }

    #[test]
    fn rejects_unknown_configuration_values() {
        let values = BTreeMap::from([("other".to_string(), Value::Bool(true))]);
        assert!(validate_values(&[], &values).is_err());
    }

    #[test]
    fn validates_select_options() {
        let mut field = field("language", ConfigurationFieldType::Select);
        field.required = true;
        field.options = vec![Value::String("en".into()), Value::String("zh".into())];
        let values = BTreeMap::from([("language".into(), Value::String("fr".into()))]);
        assert!(validate_values(&[field], &values).is_err());
    }

    #[test]
    fn validates_number_and_text_constraints() {
        let mut retries = field("retries", ConfigurationFieldType::Number);
        retries.minimum = Some(1.0);
        retries.maximum = Some(3.0);
        let mut label = field("label", ConfigurationFieldType::Text);
        label.min_length = Some(2);
        label.max_length = Some(4);

        assert!(validate_values(
            &[retries.clone(), label.clone()],
            &BTreeMap::from([
                ("retries".into(), Value::from(2)),
                ("label".into(), Value::String("工具".into())),
            ]),
        )
        .is_ok());
        assert!(validate_values(
            &[retries],
            &BTreeMap::from([("retries".into(), Value::from(4))]),
        )
        .is_err());
        assert!(validate_values(
            &[label],
            &BTreeMap::from([("label".into(), Value::String("a".into()))]),
        )
        .is_err());
    }

    #[test]
    fn migrates_config_version_and_adds_new_defaults() {
        let endpoint = field("endpoint", ConfigurationFieldType::Text);
        let mut retries = field("retries", ConfigurationFieldType::Number);
        retries.default = Some(Value::from(3));
        let stored = StoredConfiguration {
            config_version: 1,
            secret_generation: None,
            values: BTreeMap::from([(
                "endpoint".into(),
                Value::String("https://old.example".into()),
            )]),
            schema: vec![endpoint.clone()],
        };
        let descriptor = ConfigurationDescriptor {
            config_version: 2,
            owner: ConfigurationOwner::Host,
            open_command: Vec::new(),
            schema: vec![endpoint, retries],
            environment_mapping: BTreeMap::new(),
        };

        let (values, migrated) = migrate_values(&stored, &descriptor).unwrap();

        assert!(migrated);
        assert_eq!(
            values["endpoint"],
            Value::String("https://old.example".into())
        );
        assert_eq!(values["retries"], Value::from(3));
    }

    #[test]
    fn redacts_passwords_for_export() {
        let schema = vec![
            field("endpoint", ConfigurationFieldType::Text),
            field("apiKey", ConfigurationFieldType::Password),
        ];
        let values = BTreeMap::from([
            (
                "endpoint".into(),
                Value::String("https://example.com".into()),
            ),
            ("apiKey".into(), Value::String("secret".into())),
        ]);

        let exported = redact_exported_passwords(&schema, values);

        assert_eq!(exported["apiKey"], Value::String("********".into()));
        assert_eq!(
            exported["endpoint"],
            Value::String("https://example.com".into())
        );
    }

    #[test]
    fn serializes_a_portable_configuration_export_without_secrets() {
        let document = ConfigurationExportDocument {
            format_version: 1,
            extension_id: "io.example.tool".into(),
            config_version: 3,
            values: BTreeMap::from([
                (
                    "endpoint".into(),
                    Value::String("https://example.com".into()),
                ),
                ("apiKey".into(), Value::String(PASSWORD_PLACEHOLDER.into())),
            ]),
        };

        let json = serde_json::to_string(&document).unwrap();

        assert!(json.contains("\"formatVersion\":1"));
        assert!(json.contains("\"extensionId\":\"io.example.tool\""));
        assert!(json.contains("\"configVersion\":3"));
        assert!(!json.contains("secret"));
    }

    #[test]
    fn redacts_password_values_and_defaults_for_ipc() {
        let mut api_key = field("apiKey", ConfigurationFieldType::Password);
        api_key.default = Some(Value::String("default-secret".into()));
        let descriptor = ConfigurationDescriptor {
            config_version: 1,
            owner: ConfigurationOwner::Host,
            open_command: Vec::new(),
            schema: vec![api_key],
            environment_mapping: BTreeMap::new(),
        };
        let configuration = configuration_for_ipc(
            descriptor,
            BTreeMap::from([("apiKey".into(), Value::String("saved-secret".into()))]),
            None,
        );

        assert_eq!(
            configuration.values["apiKey"],
            Value::String(PASSWORD_PLACEHOLDER.into())
        );
        assert_eq!(
            configuration.descriptor.schema[0].default,
            Some(Value::String(PASSWORD_PLACEHOLDER.into()))
        );
    }

    #[test]
    fn submitted_password_placeholder_preserves_existing_secret() {
        let schema = vec![field("apiKey", ConfigurationFieldType::Password)];
        let values = preserve_password_placeholders(
            &schema,
            BTreeMap::from([("apiKey".into(), Value::String("saved-secret".into()))]),
            BTreeMap::from([("apiKey".into(), Value::String(PASSWORD_PLACEHOLDER.into()))]),
        );
        assert_eq!(values["apiKey"], Value::String("saved-secret".into()));

        let values = preserve_password_placeholders(
            &schema,
            BTreeMap::new(),
            BTreeMap::from([("apiKey".into(), Value::String(PASSWORD_PLACEHOLDER.into()))]),
        );
        assert!(!values.contains_key("apiKey"));
    }

    #[test]
    fn imported_password_placeholder_preserves_existing_secret() {
        let schema = vec![
            field("endpoint", ConfigurationFieldType::Text),
            field("apiKey", ConfigurationFieldType::Password),
        ];
        let current = BTreeMap::from([
            (
                "endpoint".into(),
                Value::String("https://old.example".into()),
            ),
            ("apiKey".into(), Value::String("local-secret".into())),
        ]);
        let imported = BTreeMap::from([
            (
                "endpoint".into(),
                Value::String("https://new.example".into()),
            ),
            ("apiKey".into(), Value::String("********".into())),
        ]);

        let (merged, changed) = merge_imported_values(&schema, current, &imported).unwrap();

        assert!(changed);
        assert_eq!(merged["apiKey"], Value::String("local-secret".into()));
        assert_eq!(
            merged["endpoint"],
            Value::String("https://new.example".into())
        );
    }

    #[test]
    fn repeated_configuration_import_is_idempotent() {
        let schema = vec![field("endpoint", ConfigurationFieldType::Text)];
        let values = BTreeMap::from([(
            "endpoint".into(),
            Value::String("https://example.com".into()),
        )]);

        let (_, changed) = merge_imported_values(&schema, values.clone(), &values).unwrap();

        assert!(!changed);
    }

    #[test]
    fn injects_env_var_mapped_configuration() {
        let directory = tempfile::tempdir().unwrap();
        let mut api_key = field("apiKey", ConfigurationFieldType::Password);
        api_key.env_var = Some("V_API_KEY".into());
        save_values(
            directory.path(),
            "io.example.tool",
            1,
            &[api_key],
            &BTreeMap::from([("apiKey".into(), Value::String("abc123".into()))]),
        )
        .unwrap();
        let mut invocation = ProviderInvocation {
            extension_id: "io.example.tool".into(),
            executable: "tool".into(),
            executable_prefix: Vec::new(),
            runtime_root: None,
            package_version: "1.0.0".into(),
            tool_version_hint: None,
            version_args: Vec::new(),
            config: crate::extensions::manifest::ProviderConfig {
                kind: crate::extensions::manifest::ProviderKind::Executable,
                descriptor: None,
                args_prefix: Vec::new(),
                describe_timeout_ms: 5_000,
                complete_timeout_ms: 800,
                environment: BTreeMap::new(),
            },
            permissions: Vec::new(),
        };

        apply_persisted_configuration(directory.path(), &mut invocation).unwrap();

        assert_eq!(invocation.config.environment["V_API_KEY"], "abc123");
        let public =
            std::fs::read_to_string(directory.path().join("io.example.tool/config.json")).unwrap();
        assert!(!public.contains("abc123"));
        assert!(public.contains("[REDACTED]"));
    }

    #[test]
    fn public_commit_failure_keeps_the_previous_generation() {
        let directory = tempfile::tempdir().unwrap();
        let schema = vec![
            field("endpoint", ConfigurationFieldType::Text),
            field("apiKey", ConfigurationFieldType::Password),
        ];
        let old = BTreeMap::from([
            ("endpoint".into(), Value::String("old".into())),
            ("apiKey".into(), Value::String("old-secret".into())),
        ]);
        save_values(directory.path(), "io.example.tool", 1, &schema, &old).unwrap();
        let new = BTreeMap::from([
            ("endpoint".into(), Value::String("new".into())),
            ("apiKey".into(), Value::String("new-secret".into())),
        ]);

        assert!(save_values_with_failure(
            directory.path(),
            "io.example.tool",
            1,
            &schema,
            &new,
            Some(SaveFailure::CommitPublic),
        )
        .is_err());

        assert_eq!(
            load_stored(directory.path(), "io.example.tool")
                .unwrap()
                .values,
            old
        );
    }

    #[cfg(unix)]
    #[test]
    fn secret_protection_failure_keeps_the_previous_generation() {
        let directory = tempfile::tempdir().unwrap();
        let schema = vec![field("apiKey", ConfigurationFieldType::Password)];
        let old = BTreeMap::from([("apiKey".into(), Value::String("old-secret".into()))]);
        save_values(directory.path(), "io.example.tool", 1, &schema, &old).unwrap();
        let new = BTreeMap::from([("apiKey".into(), Value::String("new-secret".into()))]);

        assert!(save_values_with_failure(
            directory.path(),
            "io.example.tool",
            1,
            &schema,
            &new,
            Some(SaveFailure::ProtectSecrets),
        )
        .is_err());

        assert_eq!(
            load_stored(directory.path(), "io.example.tool")
                .unwrap()
                .values,
            old
        );
    }

    #[test]
    fn startup_repair_removes_a_missing_secret_generation_reference() {
        let directory = tempfile::tempdir().unwrap();
        let schema = vec![
            field("endpoint", ConfigurationFieldType::Text),
            field("apiKey", ConfigurationFieldType::Password),
        ];
        let values = BTreeMap::from([
            ("endpoint".into(), Value::String("public".into())),
            ("apiKey".into(), Value::String("secret".into())),
        ]);
        save_values(directory.path(), "io.example.tool", 1, &schema, &values).unwrap();
        let public_path = values_path(directory.path(), "io.example.tool").unwrap();
        let stored: StoredConfiguration =
            serde_json::from_slice(&std::fs::read(&public_path).unwrap()).unwrap();
        let generation = stored.secret_generation.unwrap();
        std::fs::remove_file(
            secrets_generation_path(directory.path(), "io.example.tool", &generation).unwrap(),
        )
        .unwrap();

        recover_configurations(directory.path()).unwrap();

        let repaired = load_stored(directory.path(), "io.example.tool").unwrap();
        assert_eq!(repaired.values["endpoint"], Value::String("public".into()));
        assert!(!repaired.values.contains_key("apiKey"));
        assert!(repaired.secret_generation.is_none());
    }

    #[tokio::test(flavor = "multi_thread", worker_threads = 4)]
    async fn concurrent_saves_never_mix_public_and_secret_values() {
        let directory = tempfile::tempdir().unwrap();
        let state = std::sync::Arc::new(
            ExtensionState::from_paths(crate::extensions::ExtensionPaths::from_root(
                directory.path().to_path_buf(),
            ))
            .unwrap(),
        );
        let schema = std::sync::Arc::new(vec![
            field("marker", ConfigurationFieldType::Text),
            field("apiKey", ConfigurationFieldType::Password),
        ]);
        let tasks = (0..16)
            .map(|index| {
                let state = state.clone();
                let schema = schema.clone();
                tokio::spawn(async move {
                    let _guard = state.mutation_lock.lock().await;
                    save_values(
                        &state.paths.data,
                        "io.example.tool",
                        1,
                        &schema,
                        &BTreeMap::from([
                            ("marker".into(), Value::String(index.to_string())),
                            ("apiKey".into(), Value::String(format!("secret-{index}"))),
                        ]),
                    )
                    .unwrap();
                })
            })
            .collect::<Vec<_>>();
        for task in tasks {
            task.await.unwrap();
        }

        let stored = load_stored(&state.paths.data, "io.example.tool").unwrap();
        let marker = stored.values["marker"].as_str().unwrap();
        assert_eq!(
            stored.values["apiKey"],
            Value::String(format!("secret-{marker}"))
        );
    }
}
