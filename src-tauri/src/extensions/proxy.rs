use crate::extensions::manifest::Permission;
use std::collections::BTreeMap;

/// Proxy variables that are safe to preserve when a network-capable extension
/// runs without general access to the Host environment.
const PROXY_VARIABLES: [&str; 6] = [
    "HTTP_PROXY",
    "HTTPS_PROXY",
    "NO_PROXY",
    "http_proxy",
    "https_proxy",
    "no_proxy",
];

pub fn environment(permissions: &[Permission]) -> BTreeMap<String, String> {
    environment_from(permissions, std::env::vars())
}

fn environment_from(
    permissions: &[Permission],
    variables: impl IntoIterator<Item = (String, String)>,
) -> BTreeMap<String, String> {
    if permissions.contains(&Permission::Environment)
        || !permissions.contains(&Permission::NetworkFetch)
    {
        return BTreeMap::new();
    }

    filtered_environment(variables)
}

fn filtered_environment(
    variables: impl IntoIterator<Item = (String, String)>,
) -> BTreeMap<String, String> {
    variables
        .into_iter()
        .filter(|(name, value)| PROXY_VARIABLES.contains(&name.as_str()) && !value.is_empty())
        .collect()
}

pub fn command_environment(
    permissions: &[Permission],
    configured: &BTreeMap<String, String>,
) -> BTreeMap<String, String> {
    let mut environment = environment(permissions);
    environment.extend(configured.clone());
    environment
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn filters_proxy_environment_without_exposing_unrelated_values() {
        let environment = filtered_environment([
            ("HTTP_PROXY".into(), "http://proxy.example:8080".into()),
            (
                "https_proxy".into(),
                "http://secure-proxy.example:8080".into(),
            ),
            ("NO_PROXY".into(), "localhost,.example.test".into()),
            ("HTTPS_PROXY".into(), String::new()),
            ("API_TOKEN".into(), "secret".into()),
        ]);

        assert_eq!(environment.len(), 3);
        assert_eq!(environment["HTTP_PROXY"], "http://proxy.example:8080");
        assert_eq!(
            environment["https_proxy"],
            "http://secure-proxy.example:8080"
        );
        assert_eq!(environment["NO_PROXY"], "localhost,.example.test");
        assert!(!environment.contains_key("API_TOKEN"));
    }

    #[test]
    fn configured_values_override_inherited_proxy_values() {
        let permissions = [Permission::NetworkFetch];
        let configured = BTreeMap::from([("HTTP_PROXY".into(), "http://configured".into())]);
        let environment = command_environment(&permissions, &configured);

        assert_eq!(environment["HTTP_PROXY"], "http://configured");
    }

    #[test]
    fn proxy_exception_requires_network_without_full_environment_access() {
        let variables = || {
            [
                ("HTTPS_PROXY".into(), "http://proxy.example:8080".into()),
                ("API_TOKEN".into(), "secret".into()),
            ]
        };

        assert!(environment_from(&[], variables()).is_empty());
        assert!(environment_from(&[Permission::Environment], variables()).is_empty());
        assert_eq!(
            environment_from(&[Permission::NetworkFetch], variables()),
            BTreeMap::from([("HTTPS_PROXY".into(), "http://proxy.example:8080".into())])
        );
    }
}
