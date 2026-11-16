// @generated
impl serde::Serialize for ApplyLocationRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.location.is_some() {
            len += 1;
        }
        if !self.url.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.ApplyLocationRequest", len)?;
        if let Some(v) = self.location.as_ref() {
            struct_ser.serialize_field("location", v)?;
        }
        if !self.url.is_empty() {
            struct_ser.serialize_field("url", &self.url)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ApplyLocationRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "location",
            "url",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Location,
            Url,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "location" => Ok(GeneratedField::Location),
                            "url" => Ok(GeneratedField::Url),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ApplyLocationRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.ApplyLocationRequest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ApplyLocationRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut location__ = None;
                let mut url__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Location => {
                            if location__.is_some() {
                                return Err(serde::de::Error::duplicate_field("location"));
                            }
                            location__ = map_.next_value()?;
                        }
                        GeneratedField::Url => {
                            if url__.is_some() {
                                return Err(serde::de::Error::duplicate_field("url"));
                            }
                            url__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ApplyLocationRequest {
                    location: location__,
                    url: url__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.ApplyLocationRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ApplyLocationResponse {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.location.is_some() {
            len += 1;
        }
        if self.verdict_resolved {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.ApplyLocationResponse", len)?;
        if let Some(v) = self.location.as_ref() {
            struct_ser.serialize_field("location", v)?;
        }
        if self.verdict_resolved {
            struct_ser.serialize_field("verdictResolved", &self.verdict_resolved)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ApplyLocationResponse {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "location",
            "verdict_resolved",
            "verdictResolved",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Location,
            VerdictResolved,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "location" => Ok(GeneratedField::Location),
                            "verdictResolved" | "verdict_resolved" => Ok(GeneratedField::VerdictResolved),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ApplyLocationResponse;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.ApplyLocationResponse")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ApplyLocationResponse, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut location__ = None;
                let mut verdict_resolved__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Location => {
                            if location__.is_some() {
                                return Err(serde::de::Error::duplicate_field("location"));
                            }
                            location__ = map_.next_value()?;
                        }
                        GeneratedField::VerdictResolved => {
                            if verdict_resolved__.is_some() {
                                return Err(serde::de::Error::duplicate_field("verdictResolved"));
                            }
                            verdict_resolved__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ApplyLocationResponse {
                    location: location__,
                    verdict_resolved: verdict_resolved__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.ApplyLocationResponse", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for CloseSheetViewRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.view.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.CloseSheetViewRequest", len)?;
        if !self.view.is_empty() {
            struct_ser.serialize_field("view", &self.view)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for CloseSheetViewRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "view",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            View,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "view" => Ok(GeneratedField::View),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = CloseSheetViewRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.CloseSheetViewRequest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<CloseSheetViewRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut view__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::View => {
                            if view__.is_some() {
                                return Err(serde::de::Error::duplicate_field("view"));
                            }
                            view__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(CloseSheetViewRequest {
                    view: view__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.CloseSheetViewRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for CloseSheetViewResponse {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.closed {
            len += 1;
        }
        if self.location.is_some() {
            len += 1;
        }
        if !self.views.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.CloseSheetViewResponse", len)?;
        if self.closed {
            struct_ser.serialize_field("closed", &self.closed)?;
        }
        if let Some(v) = self.location.as_ref() {
            struct_ser.serialize_field("location", v)?;
        }
        if !self.views.is_empty() {
            struct_ser.serialize_field("views", &self.views)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for CloseSheetViewResponse {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "closed",
            "location",
            "views",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Closed,
            Location,
            Views,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "closed" => Ok(GeneratedField::Closed),
                            "location" => Ok(GeneratedField::Location),
                            "views" => Ok(GeneratedField::Views),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = CloseSheetViewResponse;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.CloseSheetViewResponse")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<CloseSheetViewResponse, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut closed__ = None;
                let mut location__ = None;
                let mut views__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Closed => {
                            if closed__.is_some() {
                                return Err(serde::de::Error::duplicate_field("closed"));
                            }
                            closed__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Location => {
                            if location__.is_some() {
                                return Err(serde::de::Error::duplicate_field("location"));
                            }
                            location__ = map_.next_value()?;
                        }
                        GeneratedField::Views => {
                            if views__.is_some() {
                                return Err(serde::de::Error::duplicate_field("views"));
                            }
                            views__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(CloseSheetViewResponse {
                    closed: closed__.unwrap_or_default(),
                    location: location__,
                    views: views__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.CloseSheetViewResponse", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for GetLocationRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let len = 0;
        let struct_ser = serializer.serialize_struct("agni.v1.viewer.GetLocationRequest", len)?;
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for GetLocationRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                            Err(serde::de::Error::unknown_field(value, FIELDS))
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = GetLocationRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.GetLocationRequest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<GetLocationRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                while map_.next_key::<GeneratedField>()?.is_some() {
                    let _ = map_.next_value::<serde::de::IgnoredAny>()?;
                }
                Ok(GetLocationRequest {
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.GetLocationRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for GetLocationResponse {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.location.is_some() {
            len += 1;
        }
        if !self.views.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.GetLocationResponse", len)?;
        if let Some(v) = self.location.as_ref() {
            struct_ser.serialize_field("location", v)?;
        }
        if !self.views.is_empty() {
            struct_ser.serialize_field("views", &self.views)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for GetLocationResponse {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "location",
            "views",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Location,
            Views,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "location" => Ok(GeneratedField::Location),
                            "views" => Ok(GeneratedField::Views),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = GetLocationResponse;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.GetLocationResponse")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<GetLocationResponse, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut location__ = None;
                let mut views__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Location => {
                            if location__.is_some() {
                                return Err(serde::de::Error::duplicate_field("location"));
                            }
                            location__ = map_.next_value()?;
                        }
                        GeneratedField::Views => {
                            if views__.is_some() {
                                return Err(serde::de::Error::duplicate_field("views"));
                            }
                            views__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(GetLocationResponse {
                    location: location__,
                    views: views__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.GetLocationResponse", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for LocateEntityRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.kind.is_empty() {
            len += 1;
        }
        if !self.subject.is_empty() {
            len += 1;
        }
        if !self.pin.is_empty() {
            len += 1;
        }
        if !self.sheet.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.LocateEntityRequest", len)?;
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if !self.subject.is_empty() {
            struct_ser.serialize_field("subject", &self.subject)?;
        }
        if !self.pin.is_empty() {
            struct_ser.serialize_field("pin", &self.pin)?;
        }
        if !self.sheet.is_empty() {
            struct_ser.serialize_field("sheet", &self.sheet)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for LocateEntityRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "kind",
            "subject",
            "pin",
            "sheet",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Kind,
            Subject,
            Pin,
            Sheet,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "kind" => Ok(GeneratedField::Kind),
                            "subject" => Ok(GeneratedField::Subject),
                            "pin" => Ok(GeneratedField::Pin),
                            "sheet" => Ok(GeneratedField::Sheet),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = LocateEntityRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.LocateEntityRequest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<LocateEntityRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut kind__ = None;
                let mut subject__ = None;
                let mut pin__ = None;
                let mut sheet__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Subject => {
                            if subject__.is_some() {
                                return Err(serde::de::Error::duplicate_field("subject"));
                            }
                            subject__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Pin => {
                            if pin__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pin"));
                            }
                            pin__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Sheet => {
                            if sheet__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sheet"));
                            }
                            sheet__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(LocateEntityRequest {
                    kind: kind__.unwrap_or_default(),
                    subject: subject__.unwrap_or_default(),
                    pin: pin__.unwrap_or_default(),
                    sheet: sheet__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.LocateEntityRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for LocateEntityResponse {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.location.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.LocateEntityResponse", len)?;
        if let Some(v) = self.location.as_ref() {
            struct_ser.serialize_field("location", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for LocateEntityResponse {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "location",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Location,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "location" => Ok(GeneratedField::Location),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = LocateEntityResponse;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.LocateEntityResponse")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<LocateEntityResponse, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut location__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Location => {
                            if location__.is_some() {
                                return Err(serde::de::Error::duplicate_field("location"));
                            }
                            location__ = map_.next_value()?;
                        }
                    }
                }
                Ok(LocateEntityResponse {
                    location: location__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.LocateEntityResponse", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for LocateVerdictRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.id.is_empty() {
            len += 1;
        }
        if !self.rule.is_empty() {
            len += 1;
        }
        if !self.subjects.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.LocateVerdictRequest", len)?;
        if !self.id.is_empty() {
            struct_ser.serialize_field("id", &self.id)?;
        }
        if !self.rule.is_empty() {
            struct_ser.serialize_field("rule", &self.rule)?;
        }
        if !self.subjects.is_empty() {
            struct_ser.serialize_field("subjects", &self.subjects)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for LocateVerdictRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "id",
            "rule",
            "subjects",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Id,
            Rule,
            Subjects,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "id" => Ok(GeneratedField::Id),
                            "rule" => Ok(GeneratedField::Rule),
                            "subjects" => Ok(GeneratedField::Subjects),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = LocateVerdictRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.LocateVerdictRequest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<LocateVerdictRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut id__ = None;
                let mut rule__ = None;
                let mut subjects__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Id => {
                            if id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("id"));
                            }
                            id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Rule => {
                            if rule__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rule"));
                            }
                            rule__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Subjects => {
                            if subjects__.is_some() {
                                return Err(serde::de::Error::duplicate_field("subjects"));
                            }
                            subjects__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(LocateVerdictRequest {
                    id: id__.unwrap_or_default(),
                    rule: rule__.unwrap_or_default(),
                    subjects: subjects__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.LocateVerdictRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for LocateVerdictResponse {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.resolved {
            len += 1;
        }
        if self.location.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.LocateVerdictResponse", len)?;
        if self.resolved {
            struct_ser.serialize_field("resolved", &self.resolved)?;
        }
        if let Some(v) = self.location.as_ref() {
            struct_ser.serialize_field("location", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for LocateVerdictResponse {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "resolved",
            "location",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Resolved,
            Location,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "resolved" => Ok(GeneratedField::Resolved),
                            "location" => Ok(GeneratedField::Location),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = LocateVerdictResponse;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.LocateVerdictResponse")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<LocateVerdictResponse, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut resolved__ = None;
                let mut location__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Resolved => {
                            if resolved__.is_some() {
                                return Err(serde::de::Error::duplicate_field("resolved"));
                            }
                            resolved__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Location => {
                            if location__.is_some() {
                                return Err(serde::de::Error::duplicate_field("location"));
                            }
                            location__ = map_.next_value()?;
                        }
                    }
                }
                Ok(LocateVerdictResponse {
                    resolved: resolved__.unwrap_or_default(),
                    location: location__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.LocateVerdictResponse", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for OpenSheetViewRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.sheet.is_empty() {
            len += 1;
        }
        if !self.renderer.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.OpenSheetViewRequest", len)?;
        if !self.sheet.is_empty() {
            struct_ser.serialize_field("sheet", &self.sheet)?;
        }
        if !self.renderer.is_empty() {
            struct_ser.serialize_field("renderer", &self.renderer)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for OpenSheetViewRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "sheet",
            "renderer",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Sheet,
            Renderer,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "sheet" => Ok(GeneratedField::Sheet),
                            "renderer" => Ok(GeneratedField::Renderer),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = OpenSheetViewRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.OpenSheetViewRequest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<OpenSheetViewRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut sheet__ = None;
                let mut renderer__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Sheet => {
                            if sheet__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sheet"));
                            }
                            sheet__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Renderer => {
                            if renderer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("renderer"));
                            }
                            renderer__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(OpenSheetViewRequest {
                    sheet: sheet__.unwrap_or_default(),
                    renderer: renderer__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.OpenSheetViewRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for OpenSheetViewResponse {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.view.is_some() {
            len += 1;
        }
        if self.location.is_some() {
            len += 1;
        }
        if !self.views.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.OpenSheetViewResponse", len)?;
        if let Some(v) = self.view.as_ref() {
            struct_ser.serialize_field("view", v)?;
        }
        if let Some(v) = self.location.as_ref() {
            struct_ser.serialize_field("location", v)?;
        }
        if !self.views.is_empty() {
            struct_ser.serialize_field("views", &self.views)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for OpenSheetViewResponse {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "view",
            "location",
            "views",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            View,
            Location,
            Views,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "view" => Ok(GeneratedField::View),
                            "location" => Ok(GeneratedField::Location),
                            "views" => Ok(GeneratedField::Views),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = OpenSheetViewResponse;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.OpenSheetViewResponse")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<OpenSheetViewResponse, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut view__ = None;
                let mut location__ = None;
                let mut views__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::View => {
                            if view__.is_some() {
                                return Err(serde::de::Error::duplicate_field("view"));
                            }
                            view__ = map_.next_value()?;
                        }
                        GeneratedField::Location => {
                            if location__.is_some() {
                                return Err(serde::de::Error::duplicate_field("location"));
                            }
                            location__ = map_.next_value()?;
                        }
                        GeneratedField::Views => {
                            if views__.is_some() {
                                return Err(serde::de::Error::duplicate_field("views"));
                            }
                            views__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(OpenSheetViewResponse {
                    view: view__,
                    location: location__,
                    views: views__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.OpenSheetViewResponse", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SelectFindingRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.subject.is_empty() {
            len += 1;
        }
        if !self.net_id.is_empty() {
            len += 1;
        }
        if !self.sheet.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.SelectFindingRequest", len)?;
        if !self.subject.is_empty() {
            struct_ser.serialize_field("subject", &self.subject)?;
        }
        if !self.net_id.is_empty() {
            struct_ser.serialize_field("netId", &self.net_id)?;
        }
        if !self.sheet.is_empty() {
            struct_ser.serialize_field("sheet", &self.sheet)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SelectFindingRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "subject",
            "net_id",
            "netId",
            "sheet",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Subject,
            NetId,
            Sheet,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "subject" => Ok(GeneratedField::Subject),
                            "netId" | "net_id" => Ok(GeneratedField::NetId),
                            "sheet" => Ok(GeneratedField::Sheet),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SelectFindingRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.SelectFindingRequest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SelectFindingRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut subject__ = None;
                let mut net_id__ = None;
                let mut sheet__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Subject => {
                            if subject__.is_some() {
                                return Err(serde::de::Error::duplicate_field("subject"));
                            }
                            subject__ = Some(map_.next_value()?);
                        }
                        GeneratedField::NetId => {
                            if net_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("netId"));
                            }
                            net_id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Sheet => {
                            if sheet__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sheet"));
                            }
                            sheet__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(SelectFindingRequest {
                    subject: subject__.unwrap_or_default(),
                    net_id: net_id__.unwrap_or_default(),
                    sheet: sheet__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.SelectFindingRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SelectFindingResponse {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.selected {
            len += 1;
        }
        if self.location.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.SelectFindingResponse", len)?;
        if self.selected {
            struct_ser.serialize_field("selected", &self.selected)?;
        }
        if let Some(v) = self.location.as_ref() {
            struct_ser.serialize_field("location", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SelectFindingResponse {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "selected",
            "location",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Selected,
            Location,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "selected" => Ok(GeneratedField::Selected),
                            "location" => Ok(GeneratedField::Location),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SelectFindingResponse;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.SelectFindingResponse")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SelectFindingResponse, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut selected__ = None;
                let mut location__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Selected => {
                            if selected__.is_some() {
                                return Err(serde::de::Error::duplicate_field("selected"));
                            }
                            selected__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Location => {
                            if location__.is_some() {
                                return Err(serde::de::Error::duplicate_field("location"));
                            }
                            location__ = map_.next_value()?;
                        }
                    }
                }
                Ok(SelectFindingResponse {
                    selected: selected__.unwrap_or_default(),
                    location: location__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.SelectFindingResponse", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SheetView {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.id.is_empty() {
            len += 1;
        }
        if !self.sheet.is_empty() {
            len += 1;
        }
        if !self.renderer.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.SheetView", len)?;
        if !self.id.is_empty() {
            struct_ser.serialize_field("id", &self.id)?;
        }
        if !self.sheet.is_empty() {
            struct_ser.serialize_field("sheet", &self.sheet)?;
        }
        if !self.renderer.is_empty() {
            struct_ser.serialize_field("renderer", &self.renderer)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SheetView {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "id",
            "sheet",
            "renderer",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Id,
            Sheet,
            Renderer,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "id" => Ok(GeneratedField::Id),
                            "sheet" => Ok(GeneratedField::Sheet),
                            "renderer" => Ok(GeneratedField::Renderer),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SheetView;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.SheetView")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SheetView, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut id__ = None;
                let mut sheet__ = None;
                let mut renderer__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Id => {
                            if id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("id"));
                            }
                            id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Sheet => {
                            if sheet__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sheet"));
                            }
                            sheet__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Renderer => {
                            if renderer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("renderer"));
                            }
                            renderer__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(SheetView {
                    id: id__.unwrap_or_default(),
                    sheet: sheet__.unwrap_or_default(),
                    renderer: renderer__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.SheetView", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ShowSheetRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.sheet.is_empty() {
            len += 1;
        }
        if !self.view.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.ShowSheetRequest", len)?;
        if !self.sheet.is_empty() {
            struct_ser.serialize_field("sheet", &self.sheet)?;
        }
        if !self.view.is_empty() {
            struct_ser.serialize_field("view", &self.view)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ShowSheetRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "sheet",
            "view",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Sheet,
            View,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "sheet" => Ok(GeneratedField::Sheet),
                            "view" => Ok(GeneratedField::View),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ShowSheetRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.ShowSheetRequest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ShowSheetRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut sheet__ = None;
                let mut view__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Sheet => {
                            if sheet__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sheet"));
                            }
                            sheet__ = Some(map_.next_value()?);
                        }
                        GeneratedField::View => {
                            if view__.is_some() {
                                return Err(serde::de::Error::duplicate_field("view"));
                            }
                            view__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ShowSheetRequest {
                    sheet: sheet__.unwrap_or_default(),
                    view: view__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.ShowSheetRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ShowSheetResponse {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.location.is_some() {
            len += 1;
        }
        if !self.views.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.ShowSheetResponse", len)?;
        if let Some(v) = self.location.as_ref() {
            struct_ser.serialize_field("location", v)?;
        }
        if !self.views.is_empty() {
            struct_ser.serialize_field("views", &self.views)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ShowSheetResponse {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "location",
            "views",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Location,
            Views,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "location" => Ok(GeneratedField::Location),
                            "views" => Ok(GeneratedField::Views),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ShowSheetResponse;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.ShowSheetResponse")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ShowSheetResponse, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut location__ = None;
                let mut views__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Location => {
                            if location__.is_some() {
                                return Err(serde::de::Error::duplicate_field("location"));
                            }
                            location__ = map_.next_value()?;
                        }
                        GeneratedField::Views => {
                            if views__.is_some() {
                                return Err(serde::de::Error::duplicate_field("views"));
                            }
                            views__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ShowSheetResponse {
                    location: location__,
                    views: views__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.ShowSheetResponse", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ViewerLocation {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.mount.is_empty() {
            len += 1;
        }
        if !self.path.is_empty() {
            len += 1;
        }
        if !self.sheet.is_empty() {
            len += 1;
        }
        if !self.mode.is_empty() {
            len += 1;
        }
        if !self.layout.is_empty() {
            len += 1;
        }
        if self.symbols.is_some() {
            len += 1;
        }
        if self.flatten.is_some() {
            len += 1;
        }
        if !self.verdict.is_empty() {
            len += 1;
        }
        if !self.hash.is_empty() {
            len += 1;
        }
        if !self.rule.is_empty() {
            len += 1;
        }
        if !self.subjects.is_empty() {
            len += 1;
        }
        if !self.trace.is_empty() {
            len += 1;
        }
        if self.trace_hops != 0 {
            len += 1;
        }
        if !self.query.is_empty() {
            len += 1;
        }
        if !self.title.is_empty() {
            len += 1;
        }
        if !self.binds.is_empty() {
            len += 1;
        }
        if !self.set.is_empty() {
            len += 1;
        }
        if !self.q.is_empty() {
            len += 1;
        }
        if !self.against.is_empty() {
            len += 1;
        }
        if self.rename_approx {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.viewer.ViewerLocation", len)?;
        if !self.mount.is_empty() {
            struct_ser.serialize_field("mount", &self.mount)?;
        }
        if !self.path.is_empty() {
            struct_ser.serialize_field("path", &self.path)?;
        }
        if !self.sheet.is_empty() {
            struct_ser.serialize_field("sheet", &self.sheet)?;
        }
        if !self.mode.is_empty() {
            struct_ser.serialize_field("mode", &self.mode)?;
        }
        if !self.layout.is_empty() {
            struct_ser.serialize_field("layout", &self.layout)?;
        }
        if let Some(v) = self.symbols.as_ref() {
            struct_ser.serialize_field("symbols", v)?;
        }
        if let Some(v) = self.flatten.as_ref() {
            struct_ser.serialize_field("flatten", v)?;
        }
        if !self.verdict.is_empty() {
            struct_ser.serialize_field("verdict", &self.verdict)?;
        }
        if !self.hash.is_empty() {
            struct_ser.serialize_field("hash", &self.hash)?;
        }
        if !self.rule.is_empty() {
            struct_ser.serialize_field("rule", &self.rule)?;
        }
        if !self.subjects.is_empty() {
            struct_ser.serialize_field("subjects", &self.subjects)?;
        }
        if !self.trace.is_empty() {
            struct_ser.serialize_field("trace", &self.trace)?;
        }
        if self.trace_hops != 0 {
            struct_ser.serialize_field("traceHops", &self.trace_hops)?;
        }
        if !self.query.is_empty() {
            struct_ser.serialize_field("query", &self.query)?;
        }
        if !self.title.is_empty() {
            struct_ser.serialize_field("title", &self.title)?;
        }
        if !self.binds.is_empty() {
            struct_ser.serialize_field("binds", &self.binds)?;
        }
        if !self.set.is_empty() {
            struct_ser.serialize_field("set", &self.set)?;
        }
        if !self.q.is_empty() {
            struct_ser.serialize_field("q", &self.q)?;
        }
        if !self.against.is_empty() {
            struct_ser.serialize_field("against", &self.against)?;
        }
        if self.rename_approx {
            struct_ser.serialize_field("renameApprox", &self.rename_approx)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ViewerLocation {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "mount",
            "path",
            "sheet",
            "mode",
            "layout",
            "symbols",
            "flatten",
            "verdict",
            "hash",
            "rule",
            "subjects",
            "trace",
            "trace_hops",
            "traceHops",
            "query",
            "title",
            "binds",
            "set",
            "q",
            "against",
            "rename_approx",
            "renameApprox",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Mount,
            Path,
            Sheet,
            Mode,
            Layout,
            Symbols,
            Flatten,
            Verdict,
            Hash,
            Rule,
            Subjects,
            Trace,
            TraceHops,
            Query,
            Title,
            Binds,
            Set,
            Q,
            Against,
            RenameApprox,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "mount" => Ok(GeneratedField::Mount),
                            "path" => Ok(GeneratedField::Path),
                            "sheet" => Ok(GeneratedField::Sheet),
                            "mode" => Ok(GeneratedField::Mode),
                            "layout" => Ok(GeneratedField::Layout),
                            "symbols" => Ok(GeneratedField::Symbols),
                            "flatten" => Ok(GeneratedField::Flatten),
                            "verdict" => Ok(GeneratedField::Verdict),
                            "hash" => Ok(GeneratedField::Hash),
                            "rule" => Ok(GeneratedField::Rule),
                            "subjects" => Ok(GeneratedField::Subjects),
                            "trace" => Ok(GeneratedField::Trace),
                            "traceHops" | "trace_hops" => Ok(GeneratedField::TraceHops),
                            "query" => Ok(GeneratedField::Query),
                            "title" => Ok(GeneratedField::Title),
                            "binds" => Ok(GeneratedField::Binds),
                            "set" => Ok(GeneratedField::Set),
                            "q" => Ok(GeneratedField::Q),
                            "against" => Ok(GeneratedField::Against),
                            "renameApprox" | "rename_approx" => Ok(GeneratedField::RenameApprox),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ViewerLocation;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.viewer.ViewerLocation")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ViewerLocation, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut mount__ = None;
                let mut path__ = None;
                let mut sheet__ = None;
                let mut mode__ = None;
                let mut layout__ = None;
                let mut symbols__ = None;
                let mut flatten__ = None;
                let mut verdict__ = None;
                let mut hash__ = None;
                let mut rule__ = None;
                let mut subjects__ = None;
                let mut trace__ = None;
                let mut trace_hops__ = None;
                let mut query__ = None;
                let mut title__ = None;
                let mut binds__ = None;
                let mut set__ = None;
                let mut q__ = None;
                let mut against__ = None;
                let mut rename_approx__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Mount => {
                            if mount__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mount"));
                            }
                            mount__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Path => {
                            if path__.is_some() {
                                return Err(serde::de::Error::duplicate_field("path"));
                            }
                            path__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Sheet => {
                            if sheet__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sheet"));
                            }
                            sheet__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Mode => {
                            if mode__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mode"));
                            }
                            mode__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Layout => {
                            if layout__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layout"));
                            }
                            layout__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Symbols => {
                            if symbols__.is_some() {
                                return Err(serde::de::Error::duplicate_field("symbols"));
                            }
                            symbols__ = map_.next_value()?;
                        }
                        GeneratedField::Flatten => {
                            if flatten__.is_some() {
                                return Err(serde::de::Error::duplicate_field("flatten"));
                            }
                            flatten__ = map_.next_value()?;
                        }
                        GeneratedField::Verdict => {
                            if verdict__.is_some() {
                                return Err(serde::de::Error::duplicate_field("verdict"));
                            }
                            verdict__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Hash => {
                            if hash__.is_some() {
                                return Err(serde::de::Error::duplicate_field("hash"));
                            }
                            hash__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Rule => {
                            if rule__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rule"));
                            }
                            rule__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Subjects => {
                            if subjects__.is_some() {
                                return Err(serde::de::Error::duplicate_field("subjects"));
                            }
                            subjects__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Trace => {
                            if trace__.is_some() {
                                return Err(serde::de::Error::duplicate_field("trace"));
                            }
                            trace__ = Some(map_.next_value()?);
                        }
                        GeneratedField::TraceHops => {
                            if trace_hops__.is_some() {
                                return Err(serde::de::Error::duplicate_field("traceHops"));
                            }
                            trace_hops__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Query => {
                            if query__.is_some() {
                                return Err(serde::de::Error::duplicate_field("query"));
                            }
                            query__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Title => {
                            if title__.is_some() {
                                return Err(serde::de::Error::duplicate_field("title"));
                            }
                            title__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Binds => {
                            if binds__.is_some() {
                                return Err(serde::de::Error::duplicate_field("binds"));
                            }
                            binds__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Set => {
                            if set__.is_some() {
                                return Err(serde::de::Error::duplicate_field("set"));
                            }
                            set__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Q => {
                            if q__.is_some() {
                                return Err(serde::de::Error::duplicate_field("q"));
                            }
                            q__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Against => {
                            if against__.is_some() {
                                return Err(serde::de::Error::duplicate_field("against"));
                            }
                            against__ = Some(map_.next_value()?);
                        }
                        GeneratedField::RenameApprox => {
                            if rename_approx__.is_some() {
                                return Err(serde::de::Error::duplicate_field("renameApprox"));
                            }
                            rename_approx__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ViewerLocation {
                    mount: mount__.unwrap_or_default(),
                    path: path__.unwrap_or_default(),
                    sheet: sheet__.unwrap_or_default(),
                    mode: mode__.unwrap_or_default(),
                    layout: layout__.unwrap_or_default(),
                    symbols: symbols__,
                    flatten: flatten__,
                    verdict: verdict__.unwrap_or_default(),
                    hash: hash__.unwrap_or_default(),
                    rule: rule__.unwrap_or_default(),
                    subjects: subjects__.unwrap_or_default(),
                    trace: trace__.unwrap_or_default(),
                    trace_hops: trace_hops__.unwrap_or_default(),
                    query: query__.unwrap_or_default(),
                    title: title__.unwrap_or_default(),
                    binds: binds__.unwrap_or_default(),
                    set: set__.unwrap_or_default(),
                    q: q__.unwrap_or_default(),
                    against: against__.unwrap_or_default(),
                    rename_approx: rename_approx__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.viewer.ViewerLocation", FIELDS, GeneratedVisitor)
    }
}
