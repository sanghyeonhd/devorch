//! Typed, ULID-backed identifiers.
//!
//! Every domain object carries its own identifier type so a `TaskId` can never
//! be passed where a `WorkspaceId` is expected. ULIDs are used because they are
//! lexicographically sortable by creation time, which keeps SQLite indexes and
//! event journals naturally ordered.

use std::fmt;
use std::str::FromStr;

use serde::{Deserialize, Serialize};

/// Error returned when a string cannot be parsed as an identifier.
#[derive(Debug, thiserror::Error)]
#[error("invalid {kind} identifier: {value}")]
pub struct IdParseError {
    pub kind: &'static str,
    pub value: String,
}

macro_rules! define_id {
    ($name:ident, $kind:literal, $prefix:literal) => {
        #[doc = concat!("Identifier for a ", $kind, ".")]
        #[derive(Clone, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
        #[serde(transparent)]
        pub struct $name(String);

        impl $name {
            /// Generate a fresh identifier.
            pub fn new() -> Self {
                Self(format!("{}_{}", $prefix, ulid::Ulid::new()))
            }

            /// Borrow the identifier as a string slice.
            pub fn as_str(&self) -> &str {
                &self.0
            }

            /// Consume the identifier, returning the owned string.
            pub fn into_string(self) -> String {
                self.0
            }
        }

        impl Default for $name {
            fn default() -> Self {
                Self::new()
            }
        }

        impl fmt::Display for $name {
            fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
                f.write_str(&self.0)
            }
        }

        impl fmt::Debug for $name {
            fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
                write!(f, "{}({})", stringify!($name), self.0)
            }
        }

        impl FromStr for $name {
            type Err = IdParseError;

            fn from_str(s: &str) -> Result<Self, Self::Err> {
                if s.is_empty() {
                    return Err(IdParseError {
                        kind: $kind,
                        value: s.to_string(),
                    });
                }
                Ok(Self(s.to_string()))
            }
        }

        impl From<$name> for String {
            fn from(id: $name) -> String {
                id.0
            }
        }
    };
}

define_id!(MissionId, "mission", "msn");
define_id!(TaskId, "task", "tsk");
define_id!(WorkspaceId, "workspace", "wsp");
define_id!(SessionId, "session", "ses");
define_id!(EventId, "event", "evt");

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn generated_ids_are_prefixed_and_unique() {
        let a = WorkspaceId::new();
        let b = WorkspaceId::new();
        assert!(a.as_str().starts_with("wsp_"));
        assert_ne!(a, b);
    }

    #[test]
    fn ids_round_trip_through_strings() {
        let id = MissionId::new();
        let parsed: MissionId = id.as_str().parse().expect("parse");
        assert_eq!(id, parsed);
    }

    #[test]
    fn empty_string_is_rejected() {
        assert!("".parse::<TaskId>().is_err());
    }

    #[test]
    fn ids_are_transparent_in_json() {
        let id = SessionId::new();
        let json = serde_json::to_string(&id).expect("serialize");
        assert_eq!(json, format!("\"{}\"", id));
    }
}
