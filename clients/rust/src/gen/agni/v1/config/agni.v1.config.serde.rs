// @generated
impl serde::Serialize for AttributeKeys {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.keys.is_empty() {
            len += 1;
        }
        if self.replace {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.AttributeKeys", len)?;
        if !self.keys.is_empty() {
            struct_ser.serialize_field("keys", &self.keys)?;
        }
        if self.replace {
            struct_ser.serialize_field("replace", &self.replace)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for AttributeKeys {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "keys",
            "replace",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Keys,
            Replace,
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
                            "keys" => Ok(GeneratedField::Keys),
                            "replace" => Ok(GeneratedField::Replace),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = AttributeKeys;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.AttributeKeys")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<AttributeKeys, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut keys__ = None;
                let mut replace__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Keys => {
                            if keys__.is_some() {
                                return Err(serde::de::Error::duplicate_field("keys"));
                            }
                            keys__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Replace => {
                            if replace__.is_some() {
                                return Err(serde::de::Error::duplicate_field("replace"));
                            }
                            replace__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(AttributeKeys {
                    keys: keys__.unwrap_or_default(),
                    replace: replace__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.AttributeKeys", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for AttributeVocab {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.mpn.is_some() {
            len += 1;
        }
        if self.value.is_some() {
            len += 1;
        }
        if self.dnp.is_some() {
            len += 1;
        }
        if self.dnp_values.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.AttributeVocab", len)?;
        if let Some(v) = self.mpn.as_ref() {
            struct_ser.serialize_field("mpn", v)?;
        }
        if let Some(v) = self.value.as_ref() {
            struct_ser.serialize_field("value", v)?;
        }
        if let Some(v) = self.dnp.as_ref() {
            struct_ser.serialize_field("dnp", v)?;
        }
        if let Some(v) = self.dnp_values.as_ref() {
            struct_ser.serialize_field("dnpValues", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for AttributeVocab {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "mpn",
            "value",
            "dnp",
            "dnp_values",
            "dnpValues",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Mpn,
            Value,
            Dnp,
            DnpValues,
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
                            "mpn" => Ok(GeneratedField::Mpn),
                            "value" => Ok(GeneratedField::Value),
                            "dnp" => Ok(GeneratedField::Dnp),
                            "dnpValues" | "dnp_values" => Ok(GeneratedField::DnpValues),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = AttributeVocab;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.AttributeVocab")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<AttributeVocab, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut mpn__ = None;
                let mut value__ = None;
                let mut dnp__ = None;
                let mut dnp_values__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Mpn => {
                            if mpn__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mpn"));
                            }
                            mpn__ = map_.next_value()?;
                        }
                        GeneratedField::Value => {
                            if value__.is_some() {
                                return Err(serde::de::Error::duplicate_field("value"));
                            }
                            value__ = map_.next_value()?;
                        }
                        GeneratedField::Dnp => {
                            if dnp__.is_some() {
                                return Err(serde::de::Error::duplicate_field("dnp"));
                            }
                            dnp__ = map_.next_value()?;
                        }
                        GeneratedField::DnpValues => {
                            if dnp_values__.is_some() {
                                return Err(serde::de::Error::duplicate_field("dnpValues"));
                            }
                            dnp_values__ = map_.next_value()?;
                        }
                    }
                }
                Ok(AttributeVocab {
                    mpn: mpn__,
                    value: value__,
                    dnp: dnp__,
                    dnp_values: dnp_values__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.AttributeVocab", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ClassVocab {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.patterns.is_empty() {
            len += 1;
        }
        if self.replace {
            len += 1;
        }
        if !self.prefixes.is_empty() {
            len += 1;
        }
        if !self.families.is_empty() {
            len += 1;
        }
        if !self.exclude_prefixes.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.ClassVocab", len)?;
        if !self.patterns.is_empty() {
            struct_ser.serialize_field("patterns", &self.patterns)?;
        }
        if self.replace {
            struct_ser.serialize_field("replace", &self.replace)?;
        }
        if !self.prefixes.is_empty() {
            struct_ser.serialize_field("prefixes", &self.prefixes)?;
        }
        if !self.families.is_empty() {
            struct_ser.serialize_field("families", &self.families)?;
        }
        if !self.exclude_prefixes.is_empty() {
            struct_ser.serialize_field("excludePrefixes", &self.exclude_prefixes)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ClassVocab {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "patterns",
            "replace",
            "prefixes",
            "families",
            "exclude_prefixes",
            "excludePrefixes",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Patterns,
            Replace,
            Prefixes,
            Families,
            ExcludePrefixes,
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
                            "patterns" => Ok(GeneratedField::Patterns),
                            "replace" => Ok(GeneratedField::Replace),
                            "prefixes" => Ok(GeneratedField::Prefixes),
                            "families" => Ok(GeneratedField::Families),
                            "excludePrefixes" | "exclude_prefixes" => Ok(GeneratedField::ExcludePrefixes),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ClassVocab;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.ClassVocab")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ClassVocab, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut patterns__ = None;
                let mut replace__ = None;
                let mut prefixes__ = None;
                let mut families__ = None;
                let mut exclude_prefixes__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Patterns => {
                            if patterns__.is_some() {
                                return Err(serde::de::Error::duplicate_field("patterns"));
                            }
                            patterns__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Replace => {
                            if replace__.is_some() {
                                return Err(serde::de::Error::duplicate_field("replace"));
                            }
                            replace__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Prefixes => {
                            if prefixes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prefixes"));
                            }
                            prefixes__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Families => {
                            if families__.is_some() {
                                return Err(serde::de::Error::duplicate_field("families"));
                            }
                            families__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ExcludePrefixes => {
                            if exclude_prefixes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("excludePrefixes"));
                            }
                            exclude_prefixes__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ClassVocab {
                    patterns: patterns__.unwrap_or_default(),
                    replace: replace__.unwrap_or_default(),
                    prefixes: prefixes__.unwrap_or_default(),
                    families: families__.unwrap_or_default(),
                    exclude_prefixes: exclude_prefixes__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.ClassVocab", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ComponentIntent {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.exposure.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.ComponentIntent", len)?;
        if !self.exposure.is_empty() {
            struct_ser.serialize_field("exposure", &self.exposure)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ComponentIntent {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "exposure",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Exposure,
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
                            "exposure" => Ok(GeneratedField::Exposure),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ComponentIntent;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.ComponentIntent")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ComponentIntent, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut exposure__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Exposure => {
                            if exposure__.is_some() {
                                return Err(serde::de::Error::duplicate_field("exposure"));
                            }
                            exposure__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ComponentIntent {
                    exposure: exposure__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.ComponentIntent", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DesignIntent {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.modules.is_empty() {
            len += 1;
        }
        if !self.nets.is_empty() {
            len += 1;
        }
        if !self.sequences.is_empty() {
            len += 1;
        }
        if !self.strap_groups.is_empty() {
            len += 1;
        }
        if !self.io_map.is_empty() {
            len += 1;
        }
        if self.margin_factor != 0. {
            len += 1;
        }
        if !self.components.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.DesignIntent", len)?;
        if !self.modules.is_empty() {
            struct_ser.serialize_field("modules", &self.modules)?;
        }
        if !self.nets.is_empty() {
            struct_ser.serialize_field("nets", &self.nets)?;
        }
        if !self.sequences.is_empty() {
            struct_ser.serialize_field("sequences", &self.sequences)?;
        }
        if !self.strap_groups.is_empty() {
            struct_ser.serialize_field("strapGroups", &self.strap_groups)?;
        }
        if !self.io_map.is_empty() {
            struct_ser.serialize_field("ioMap", &self.io_map)?;
        }
        if self.margin_factor != 0. {
            struct_ser.serialize_field("marginFactor", &self.margin_factor)?;
        }
        if !self.components.is_empty() {
            struct_ser.serialize_field("components", &self.components)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DesignIntent {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "modules",
            "nets",
            "sequences",
            "strap_groups",
            "strapGroups",
            "io_map",
            "ioMap",
            "margin_factor",
            "marginFactor",
            "components",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Modules,
            Nets,
            Sequences,
            StrapGroups,
            IoMap,
            MarginFactor,
            Components,
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
                            "modules" => Ok(GeneratedField::Modules),
                            "nets" => Ok(GeneratedField::Nets),
                            "sequences" => Ok(GeneratedField::Sequences),
                            "strapGroups" | "strap_groups" => Ok(GeneratedField::StrapGroups),
                            "ioMap" | "io_map" => Ok(GeneratedField::IoMap),
                            "marginFactor" | "margin_factor" => Ok(GeneratedField::MarginFactor),
                            "components" => Ok(GeneratedField::Components),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DesignIntent;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.DesignIntent")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DesignIntent, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut modules__ = None;
                let mut nets__ = None;
                let mut sequences__ = None;
                let mut strap_groups__ = None;
                let mut io_map__ = None;
                let mut margin_factor__ = None;
                let mut components__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Modules => {
                            if modules__.is_some() {
                                return Err(serde::de::Error::duplicate_field("modules"));
                            }
                            modules__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Nets => {
                            if nets__.is_some() {
                                return Err(serde::de::Error::duplicate_field("nets"));
                            }
                            nets__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                        GeneratedField::Sequences => {
                            if sequences__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sequences"));
                            }
                            sequences__ = Some(map_.next_value()?);
                        }
                        GeneratedField::StrapGroups => {
                            if strap_groups__.is_some() {
                                return Err(serde::de::Error::duplicate_field("strapGroups"));
                            }
                            strap_groups__ = Some(map_.next_value()?);
                        }
                        GeneratedField::IoMap => {
                            if io_map__.is_some() {
                                return Err(serde::de::Error::duplicate_field("ioMap"));
                            }
                            io_map__ = Some(map_.next_value()?);
                        }
                        GeneratedField::MarginFactor => {
                            if margin_factor__.is_some() {
                                return Err(serde::de::Error::duplicate_field("marginFactor"));
                            }
                            margin_factor__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Components => {
                            if components__.is_some() {
                                return Err(serde::de::Error::duplicate_field("components"));
                            }
                            components__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                    }
                }
                Ok(DesignIntent {
                    modules: modules__.unwrap_or_default(),
                    nets: nets__.unwrap_or_default(),
                    sequences: sequences__.unwrap_or_default(),
                    strap_groups: strap_groups__.unwrap_or_default(),
                    io_map: io_map__.unwrap_or_default(),
                    margin_factor: margin_factor__.unwrap_or_default(),
                    components: components__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.DesignIntent", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DiffPairVocab {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.pairs.is_empty() {
            len += 1;
        }
        if self.replace {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.DiffPairVocab", len)?;
        if !self.pairs.is_empty() {
            struct_ser.serialize_field("pairs", &self.pairs)?;
        }
        if self.replace {
            struct_ser.serialize_field("replace", &self.replace)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DiffPairVocab {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "pairs",
            "replace",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Pairs,
            Replace,
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
                            "pairs" => Ok(GeneratedField::Pairs),
                            "replace" => Ok(GeneratedField::Replace),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DiffPairVocab;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.DiffPairVocab")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DiffPairVocab, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut pairs__ = None;
                let mut replace__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Pairs => {
                            if pairs__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pairs"));
                            }
                            pairs__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Replace => {
                            if replace__.is_some() {
                                return Err(serde::de::Error::duplicate_field("replace"));
                            }
                            replace__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(DiffPairVocab {
                    pairs: pairs__.unwrap_or_default(),
                    replace: replace__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.DiffPairVocab", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DiffSuffix {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.positive.is_empty() {
            len += 1;
        }
        if !self.negative.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.DiffSuffix", len)?;
        if !self.positive.is_empty() {
            struct_ser.serialize_field("positive", &self.positive)?;
        }
        if !self.negative.is_empty() {
            struct_ser.serialize_field("negative", &self.negative)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DiffSuffix {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "positive",
            "negative",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Positive,
            Negative,
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
                            "positive" => Ok(GeneratedField::Positive),
                            "negative" => Ok(GeneratedField::Negative),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DiffSuffix;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.DiffSuffix")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DiffSuffix, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut positive__ = None;
                let mut negative__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Positive => {
                            if positive__.is_some() {
                                return Err(serde::de::Error::duplicate_field("positive"));
                            }
                            positive__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Negative => {
                            if negative__.is_some() {
                                return Err(serde::de::Error::duplicate_field("negative"));
                            }
                            negative__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(DiffSuffix {
                    positive: positive__.unwrap_or_default(),
                    negative: negative__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.DiffSuffix", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for IntentModule {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.name.is_empty() {
            len += 1;
        }
        if !self.class.is_empty() {
            len += 1;
        }
        if !self.mpn.is_empty() {
            len += 1;
        }
        if self.count != 0 {
            len += 1;
        }
        if !self.nets.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.IntentModule", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.class.is_empty() {
            struct_ser.serialize_field("class", &self.class)?;
        }
        if !self.mpn.is_empty() {
            struct_ser.serialize_field("mpn", &self.mpn)?;
        }
        if self.count != 0 {
            struct_ser.serialize_field("count", &self.count)?;
        }
        if !self.nets.is_empty() {
            struct_ser.serialize_field("nets", &self.nets)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for IntentModule {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "class",
            "mpn",
            "count",
            "nets",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Class,
            Mpn,
            Count,
            Nets,
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
                            "name" => Ok(GeneratedField::Name),
                            "class" => Ok(GeneratedField::Class),
                            "mpn" => Ok(GeneratedField::Mpn),
                            "count" => Ok(GeneratedField::Count),
                            "nets" => Ok(GeneratedField::Nets),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = IntentModule;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.IntentModule")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<IntentModule, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut class__ = None;
                let mut mpn__ = None;
                let mut count__ = None;
                let mut nets__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Class => {
                            if class__.is_some() {
                                return Err(serde::de::Error::duplicate_field("class"));
                            }
                            class__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Mpn => {
                            if mpn__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mpn"));
                            }
                            mpn__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Count => {
                            if count__.is_some() {
                                return Err(serde::de::Error::duplicate_field("count"));
                            }
                            count__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Nets => {
                            if nets__.is_some() {
                                return Err(serde::de::Error::duplicate_field("nets"));
                            }
                            nets__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(IntentModule {
                    name: name__.unwrap_or_default(),
                    class: class__.unwrap_or_default(),
                    mpn: mpn__.unwrap_or_default(),
                    count: count__.unwrap_or_default(),
                    nets: nets__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.IntentModule", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for NamingConvention {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.name.is_empty() {
            len += 1;
        }
        if self.lexicon.is_some() {
            len += 1;
        }
        if !self.rules.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.NamingConvention", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if let Some(v) = self.lexicon.as_ref() {
            struct_ser.serialize_field("lexicon", v)?;
        }
        if !self.rules.is_empty() {
            struct_ser.serialize_field("rules", &self.rules)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for NamingConvention {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "lexicon",
            "rules",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Lexicon,
            Rules,
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
                            "name" => Ok(GeneratedField::Name),
                            "lexicon" => Ok(GeneratedField::Lexicon),
                            "rules" => Ok(GeneratedField::Rules),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = NamingConvention;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.NamingConvention")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<NamingConvention, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut lexicon__ = None;
                let mut rules__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Lexicon => {
                            if lexicon__.is_some() {
                                return Err(serde::de::Error::duplicate_field("lexicon"));
                            }
                            lexicon__ = map_.next_value()?;
                        }
                        GeneratedField::Rules => {
                            if rules__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rules"));
                            }
                            rules__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(NamingConvention {
                    name: name__.unwrap_or_default(),
                    lexicon: lexicon__,
                    rules: rules__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.NamingConvention", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for NamingLexicon {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.net.is_some() {
            len += 1;
        }
        if self.pin.is_some() {
            len += 1;
        }
        if !self.class.is_empty() {
            len += 1;
        }
        if !self.value_unit.is_empty() {
            len += 1;
        }
        if self.attribute.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.NamingLexicon", len)?;
        if let Some(v) = self.net.as_ref() {
            struct_ser.serialize_field("net", v)?;
        }
        if let Some(v) = self.pin.as_ref() {
            struct_ser.serialize_field("pin", v)?;
        }
        if !self.class.is_empty() {
            struct_ser.serialize_field("class", &self.class)?;
        }
        if !self.value_unit.is_empty() {
            struct_ser.serialize_field("valueUnit", &self.value_unit)?;
        }
        if let Some(v) = self.attribute.as_ref() {
            struct_ser.serialize_field("attribute", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for NamingLexicon {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "net",
            "pin",
            "class",
            "value_unit",
            "valueUnit",
            "attribute",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Net,
            Pin,
            Class,
            ValueUnit,
            Attribute,
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
                            "net" => Ok(GeneratedField::Net),
                            "pin" => Ok(GeneratedField::Pin),
                            "class" => Ok(GeneratedField::Class),
                            "valueUnit" | "value_unit" => Ok(GeneratedField::ValueUnit),
                            "attribute" => Ok(GeneratedField::Attribute),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = NamingLexicon;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.NamingLexicon")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<NamingLexicon, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut net__ = None;
                let mut pin__ = None;
                let mut class__ = None;
                let mut value_unit__ = None;
                let mut attribute__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Net => {
                            if net__.is_some() {
                                return Err(serde::de::Error::duplicate_field("net"));
                            }
                            net__ = map_.next_value()?;
                        }
                        GeneratedField::Pin => {
                            if pin__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pin"));
                            }
                            pin__ = map_.next_value()?;
                        }
                        GeneratedField::Class => {
                            if class__.is_some() {
                                return Err(serde::de::Error::duplicate_field("class"));
                            }
                            class__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                        GeneratedField::ValueUnit => {
                            if value_unit__.is_some() {
                                return Err(serde::de::Error::duplicate_field("valueUnit"));
                            }
                            value_unit__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                        GeneratedField::Attribute => {
                            if attribute__.is_some() {
                                return Err(serde::de::Error::duplicate_field("attribute"));
                            }
                            attribute__ = map_.next_value()?;
                        }
                    }
                }
                Ok(NamingLexicon {
                    net: net__,
                    pin: pin__,
                    class: class__.unwrap_or_default(),
                    value_unit: value_unit__.unwrap_or_default(),
                    attribute: attribute__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.NamingLexicon", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for NamingRule {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.name.is_empty() {
            len += 1;
        }
        if !self.severity.is_empty() {
            len += 1;
        }
        if !self.why.is_empty() {
            len += 1;
        }
        if !self.allow.is_empty() {
            len += 1;
        }
        if !self.exempt.is_empty() {
            len += 1;
        }
        if self.match_full {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.NamingRule", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.severity.is_empty() {
            struct_ser.serialize_field("severity", &self.severity)?;
        }
        if !self.why.is_empty() {
            struct_ser.serialize_field("why", &self.why)?;
        }
        if !self.allow.is_empty() {
            struct_ser.serialize_field("allow", &self.allow)?;
        }
        if !self.exempt.is_empty() {
            struct_ser.serialize_field("exempt", &self.exempt)?;
        }
        if self.match_full {
            struct_ser.serialize_field("matchFull", &self.match_full)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for NamingRule {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "severity",
            "why",
            "allow",
            "exempt",
            "match_full",
            "matchFull",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Severity,
            Why,
            Allow,
            Exempt,
            MatchFull,
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
                            "name" => Ok(GeneratedField::Name),
                            "severity" => Ok(GeneratedField::Severity),
                            "why" => Ok(GeneratedField::Why),
                            "allow" => Ok(GeneratedField::Allow),
                            "exempt" => Ok(GeneratedField::Exempt),
                            "matchFull" | "match_full" => Ok(GeneratedField::MatchFull),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = NamingRule;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.NamingRule")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<NamingRule, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut severity__ = None;
                let mut why__ = None;
                let mut allow__ = None;
                let mut exempt__ = None;
                let mut match_full__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Severity => {
                            if severity__.is_some() {
                                return Err(serde::de::Error::duplicate_field("severity"));
                            }
                            severity__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Why => {
                            if why__.is_some() {
                                return Err(serde::de::Error::duplicate_field("why"));
                            }
                            why__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Allow => {
                            if allow__.is_some() {
                                return Err(serde::de::Error::duplicate_field("allow"));
                            }
                            allow__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Exempt => {
                            if exempt__.is_some() {
                                return Err(serde::de::Error::duplicate_field("exempt"));
                            }
                            exempt__ = Some(map_.next_value()?);
                        }
                        GeneratedField::MatchFull => {
                            if match_full__.is_some() {
                                return Err(serde::de::Error::duplicate_field("matchFull"));
                            }
                            match_full__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(NamingRule {
                    name: name__.unwrap_or_default(),
                    severity: severity__.unwrap_or_default(),
                    why: why__.unwrap_or_default(),
                    allow: allow__.unwrap_or_default(),
                    exempt: exempt__.unwrap_or_default(),
                    match_full: match_full__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.NamingRule", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for NetIntent {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.nominal.is_some() {
            len += 1;
        }
        if !self.domain.is_empty() {
            len += 1;
        }
        if self.peak.is_some() {
            len += 1;
        }
        if !self.protect.is_empty() {
            len += 1;
        }
        if !self.reset.is_empty() {
            len += 1;
        }
        if !self.strap.is_empty() {
            len += 1;
        }
        if self.min_ohms != 0. {
            len += 1;
        }
        if self.max_ohms != 0. {
            len += 1;
        }
        if self.ac_coupled {
            len += 1;
        }
        if self.pull_up_reach_hops != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.NetIntent", len)?;
        if let Some(v) = self.nominal.as_ref() {
            struct_ser.serialize_field("nominal", v)?;
        }
        if !self.domain.is_empty() {
            struct_ser.serialize_field("domain", &self.domain)?;
        }
        if let Some(v) = self.peak.as_ref() {
            struct_ser.serialize_field("peak", v)?;
        }
        if !self.protect.is_empty() {
            struct_ser.serialize_field("protect", &self.protect)?;
        }
        if !self.reset.is_empty() {
            struct_ser.serialize_field("reset", &self.reset)?;
        }
        if !self.strap.is_empty() {
            struct_ser.serialize_field("strap", &self.strap)?;
        }
        if self.min_ohms != 0. {
            struct_ser.serialize_field("minOhms", &self.min_ohms)?;
        }
        if self.max_ohms != 0. {
            struct_ser.serialize_field("maxOhms", &self.max_ohms)?;
        }
        if self.ac_coupled {
            struct_ser.serialize_field("acCoupled", &self.ac_coupled)?;
        }
        if self.pull_up_reach_hops != 0 {
            struct_ser.serialize_field("pullUpReachHops", &self.pull_up_reach_hops)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for NetIntent {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "nominal",
            "domain",
            "peak",
            "protect",
            "reset",
            "strap",
            "min_ohms",
            "minOhms",
            "max_ohms",
            "maxOhms",
            "ac_coupled",
            "acCoupled",
            "pull_up_reach_hops",
            "pullUpReachHops",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Nominal,
            Domain,
            Peak,
            Protect,
            Reset,
            Strap,
            MinOhms,
            MaxOhms,
            AcCoupled,
            PullUpReachHops,
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
                            "nominal" => Ok(GeneratedField::Nominal),
                            "domain" => Ok(GeneratedField::Domain),
                            "peak" => Ok(GeneratedField::Peak),
                            "protect" => Ok(GeneratedField::Protect),
                            "reset" => Ok(GeneratedField::Reset),
                            "strap" => Ok(GeneratedField::Strap),
                            "minOhms" | "min_ohms" => Ok(GeneratedField::MinOhms),
                            "maxOhms" | "max_ohms" => Ok(GeneratedField::MaxOhms),
                            "acCoupled" | "ac_coupled" => Ok(GeneratedField::AcCoupled),
                            "pullUpReachHops" | "pull_up_reach_hops" => Ok(GeneratedField::PullUpReachHops),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = NetIntent;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.NetIntent")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<NetIntent, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut nominal__ = None;
                let mut domain__ = None;
                let mut peak__ = None;
                let mut protect__ = None;
                let mut reset__ = None;
                let mut strap__ = None;
                let mut min_ohms__ = None;
                let mut max_ohms__ = None;
                let mut ac_coupled__ = None;
                let mut pull_up_reach_hops__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Nominal => {
                            if nominal__.is_some() {
                                return Err(serde::de::Error::duplicate_field("nominal"));
                            }
                            nominal__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                        GeneratedField::Domain => {
                            if domain__.is_some() {
                                return Err(serde::de::Error::duplicate_field("domain"));
                            }
                            domain__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Peak => {
                            if peak__.is_some() {
                                return Err(serde::de::Error::duplicate_field("peak"));
                            }
                            peak__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                        GeneratedField::Protect => {
                            if protect__.is_some() {
                                return Err(serde::de::Error::duplicate_field("protect"));
                            }
                            protect__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Reset => {
                            if reset__.is_some() {
                                return Err(serde::de::Error::duplicate_field("reset"));
                            }
                            reset__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Strap => {
                            if strap__.is_some() {
                                return Err(serde::de::Error::duplicate_field("strap"));
                            }
                            strap__ = Some(map_.next_value()?);
                        }
                        GeneratedField::MinOhms => {
                            if min_ohms__.is_some() {
                                return Err(serde::de::Error::duplicate_field("minOhms"));
                            }
                            min_ohms__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::MaxOhms => {
                            if max_ohms__.is_some() {
                                return Err(serde::de::Error::duplicate_field("maxOhms"));
                            }
                            max_ohms__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::AcCoupled => {
                            if ac_coupled__.is_some() {
                                return Err(serde::de::Error::duplicate_field("acCoupled"));
                            }
                            ac_coupled__ = Some(map_.next_value()?);
                        }
                        GeneratedField::PullUpReachHops => {
                            if pull_up_reach_hops__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pullUpReachHops"));
                            }
                            pull_up_reach_hops__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(NetIntent {
                    nominal: nominal__,
                    domain: domain__.unwrap_or_default(),
                    peak: peak__,
                    protect: protect__.unwrap_or_default(),
                    reset: reset__.unwrap_or_default(),
                    strap: strap__.unwrap_or_default(),
                    min_ohms: min_ohms__.unwrap_or_default(),
                    max_ohms: max_ohms__.unwrap_or_default(),
                    ac_coupled: ac_coupled__.unwrap_or_default(),
                    pull_up_reach_hops: pull_up_reach_hops__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.NetIntent", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for NetNameVocab {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.rail.is_some() {
            len += 1;
        }
        if self.ground.is_some() {
            len += 1;
        }
        if self.feedback.is_some() {
            len += 1;
        }
        if self.switching.is_some() {
            len += 1;
        }
        if self.control.is_some() {
            len += 1;
        }
        if self.gate_drive.is_some() {
            len += 1;
        }
        if self.i2c.is_some() {
            len += 1;
        }
        if self.infer_rail_from_supply_pins.is_some() {
            len += 1;
        }
        if self.no_connect.is_some() {
            len += 1;
        }
        if self.diff_pair.is_some() {
            len += 1;
        }
        if self.voltage.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.NetNameVocab", len)?;
        if let Some(v) = self.rail.as_ref() {
            struct_ser.serialize_field("rail", v)?;
        }
        if let Some(v) = self.ground.as_ref() {
            struct_ser.serialize_field("ground", v)?;
        }
        if let Some(v) = self.feedback.as_ref() {
            struct_ser.serialize_field("feedback", v)?;
        }
        if let Some(v) = self.switching.as_ref() {
            struct_ser.serialize_field("switching", v)?;
        }
        if let Some(v) = self.control.as_ref() {
            struct_ser.serialize_field("control", v)?;
        }
        if let Some(v) = self.gate_drive.as_ref() {
            struct_ser.serialize_field("gateDrive", v)?;
        }
        if let Some(v) = self.i2c.as_ref() {
            struct_ser.serialize_field("i2c", v)?;
        }
        if let Some(v) = self.infer_rail_from_supply_pins.as_ref() {
            struct_ser.serialize_field("inferRailFromSupplyPins", v)?;
        }
        if let Some(v) = self.no_connect.as_ref() {
            struct_ser.serialize_field("noConnect", v)?;
        }
        if let Some(v) = self.diff_pair.as_ref() {
            struct_ser.serialize_field("diffPair", v)?;
        }
        if let Some(v) = self.voltage.as_ref() {
            struct_ser.serialize_field("voltage", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for NetNameVocab {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "rail",
            "ground",
            "feedback",
            "switching",
            "control",
            "gate_drive",
            "gateDrive",
            "i2c",
            "infer_rail_from_supply_pins",
            "inferRailFromSupplyPins",
            "no_connect",
            "noConnect",
            "diff_pair",
            "diffPair",
            "voltage",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Rail,
            Ground,
            Feedback,
            Switching,
            Control,
            GateDrive,
            I2c,
            InferRailFromSupplyPins,
            NoConnect,
            DiffPair,
            Voltage,
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
                            "rail" => Ok(GeneratedField::Rail),
                            "ground" => Ok(GeneratedField::Ground),
                            "feedback" => Ok(GeneratedField::Feedback),
                            "switching" => Ok(GeneratedField::Switching),
                            "control" => Ok(GeneratedField::Control),
                            "gateDrive" | "gate_drive" => Ok(GeneratedField::GateDrive),
                            "i2c" => Ok(GeneratedField::I2c),
                            "inferRailFromSupplyPins" | "infer_rail_from_supply_pins" => Ok(GeneratedField::InferRailFromSupplyPins),
                            "noConnect" | "no_connect" => Ok(GeneratedField::NoConnect),
                            "diffPair" | "diff_pair" => Ok(GeneratedField::DiffPair),
                            "voltage" => Ok(GeneratedField::Voltage),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = NetNameVocab;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.NetNameVocab")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<NetNameVocab, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut rail__ = None;
                let mut ground__ = None;
                let mut feedback__ = None;
                let mut switching__ = None;
                let mut control__ = None;
                let mut gate_drive__ = None;
                let mut i2c__ = None;
                let mut infer_rail_from_supply_pins__ = None;
                let mut no_connect__ = None;
                let mut diff_pair__ = None;
                let mut voltage__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Rail => {
                            if rail__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rail"));
                            }
                            rail__ = map_.next_value()?;
                        }
                        GeneratedField::Ground => {
                            if ground__.is_some() {
                                return Err(serde::de::Error::duplicate_field("ground"));
                            }
                            ground__ = map_.next_value()?;
                        }
                        GeneratedField::Feedback => {
                            if feedback__.is_some() {
                                return Err(serde::de::Error::duplicate_field("feedback"));
                            }
                            feedback__ = map_.next_value()?;
                        }
                        GeneratedField::Switching => {
                            if switching__.is_some() {
                                return Err(serde::de::Error::duplicate_field("switching"));
                            }
                            switching__ = map_.next_value()?;
                        }
                        GeneratedField::Control => {
                            if control__.is_some() {
                                return Err(serde::de::Error::duplicate_field("control"));
                            }
                            control__ = map_.next_value()?;
                        }
                        GeneratedField::GateDrive => {
                            if gate_drive__.is_some() {
                                return Err(serde::de::Error::duplicate_field("gateDrive"));
                            }
                            gate_drive__ = map_.next_value()?;
                        }
                        GeneratedField::I2c => {
                            if i2c__.is_some() {
                                return Err(serde::de::Error::duplicate_field("i2c"));
                            }
                            i2c__ = map_.next_value()?;
                        }
                        GeneratedField::InferRailFromSupplyPins => {
                            if infer_rail_from_supply_pins__.is_some() {
                                return Err(serde::de::Error::duplicate_field("inferRailFromSupplyPins"));
                            }
                            infer_rail_from_supply_pins__ = map_.next_value()?;
                        }
                        GeneratedField::NoConnect => {
                            if no_connect__.is_some() {
                                return Err(serde::de::Error::duplicate_field("noConnect"));
                            }
                            no_connect__ = map_.next_value()?;
                        }
                        GeneratedField::DiffPair => {
                            if diff_pair__.is_some() {
                                return Err(serde::de::Error::duplicate_field("diffPair"));
                            }
                            diff_pair__ = map_.next_value()?;
                        }
                        GeneratedField::Voltage => {
                            if voltage__.is_some() {
                                return Err(serde::de::Error::duplicate_field("voltage"));
                            }
                            voltage__ = map_.next_value()?;
                        }
                    }
                }
                Ok(NetNameVocab {
                    rail: rail__,
                    ground: ground__,
                    feedback: feedback__,
                    switching: switching__,
                    control: control__,
                    gate_drive: gate_drive__,
                    i2c: i2c__,
                    infer_rail_from_supply_pins: infer_rail_from_supply_pins__,
                    no_connect: no_connect__,
                    diff_pair: diff_pair__,
                    voltage: voltage__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.NetNameVocab", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PinAssignment {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.net.is_empty() {
            len += 1;
        }
        if !self.device.is_empty() {
            len += 1;
        }
        if !self.pin.is_empty() {
            len += 1;
        }
        if !self.function.is_empty() {
            len += 1;
        }
        if self.to.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.PinAssignment", len)?;
        if !self.net.is_empty() {
            struct_ser.serialize_field("net", &self.net)?;
        }
        if !self.device.is_empty() {
            struct_ser.serialize_field("device", &self.device)?;
        }
        if !self.pin.is_empty() {
            struct_ser.serialize_field("pin", &self.pin)?;
        }
        if !self.function.is_empty() {
            struct_ser.serialize_field("function", &self.function)?;
        }
        if let Some(v) = self.to.as_ref() {
            struct_ser.serialize_field("to", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PinAssignment {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "net",
            "device",
            "pin",
            "function",
            "to",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Net,
            Device,
            Pin,
            Function,
            To,
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
                            "net" => Ok(GeneratedField::Net),
                            "device" => Ok(GeneratedField::Device),
                            "pin" => Ok(GeneratedField::Pin),
                            "function" => Ok(GeneratedField::Function),
                            "to" => Ok(GeneratedField::To),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PinAssignment;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.PinAssignment")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PinAssignment, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut net__ = None;
                let mut device__ = None;
                let mut pin__ = None;
                let mut function__ = None;
                let mut to__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Net => {
                            if net__.is_some() {
                                return Err(serde::de::Error::duplicate_field("net"));
                            }
                            net__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Device => {
                            if device__.is_some() {
                                return Err(serde::de::Error::duplicate_field("device"));
                            }
                            device__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Pin => {
                            if pin__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pin"));
                            }
                            pin__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Function => {
                            if function__.is_some() {
                                return Err(serde::de::Error::duplicate_field("function"));
                            }
                            function__ = Some(map_.next_value()?);
                        }
                        GeneratedField::To => {
                            if to__.is_some() {
                                return Err(serde::de::Error::duplicate_field("to"));
                            }
                            to__ = map_.next_value()?;
                        }
                    }
                }
                Ok(PinAssignment {
                    net: net__.unwrap_or_default(),
                    device: device__.unwrap_or_default(),
                    pin: pin__.unwrap_or_default(),
                    function: function__.unwrap_or_default(),
                    to: to__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.PinAssignment", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PinEndpoint {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.device.is_empty() {
            len += 1;
        }
        if !self.pin.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.PinEndpoint", len)?;
        if !self.device.is_empty() {
            struct_ser.serialize_field("device", &self.device)?;
        }
        if !self.pin.is_empty() {
            struct_ser.serialize_field("pin", &self.pin)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PinEndpoint {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "device",
            "pin",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Device,
            Pin,
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
                            "device" => Ok(GeneratedField::Device),
                            "pin" => Ok(GeneratedField::Pin),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PinEndpoint;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.PinEndpoint")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PinEndpoint, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut device__ = None;
                let mut pin__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Device => {
                            if device__.is_some() {
                                return Err(serde::de::Error::duplicate_field("device"));
                            }
                            device__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Pin => {
                            if pin__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pin"));
                            }
                            pin__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(PinEndpoint {
                    device: device__.unwrap_or_default(),
                    pin: pin__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.PinEndpoint", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PinNameVocab {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.supply.is_some() {
            len += 1;
        }
        if self.gate.is_some() {
            len += 1;
        }
        if self.source.is_some() {
            len += 1;
        }
        if self.drain.is_some() {
            len += 1;
        }
        if self.exposed_pad.is_some() {
            len += 1;
        }
        if self.anode.is_some() {
            len += 1;
        }
        if self.cathode.is_some() {
            len += 1;
        }
        if self.feedback.is_some() {
            len += 1;
        }
        if self.switching.is_some() {
            len += 1;
        }
        if self.through_pair.is_some() {
            len += 1;
        }
        if self.cable_supply.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.PinNameVocab", len)?;
        if let Some(v) = self.supply.as_ref() {
            struct_ser.serialize_field("supply", v)?;
        }
        if let Some(v) = self.gate.as_ref() {
            struct_ser.serialize_field("gate", v)?;
        }
        if let Some(v) = self.source.as_ref() {
            struct_ser.serialize_field("source", v)?;
        }
        if let Some(v) = self.drain.as_ref() {
            struct_ser.serialize_field("drain", v)?;
        }
        if let Some(v) = self.exposed_pad.as_ref() {
            struct_ser.serialize_field("exposedPad", v)?;
        }
        if let Some(v) = self.anode.as_ref() {
            struct_ser.serialize_field("anode", v)?;
        }
        if let Some(v) = self.cathode.as_ref() {
            struct_ser.serialize_field("cathode", v)?;
        }
        if let Some(v) = self.feedback.as_ref() {
            struct_ser.serialize_field("feedback", v)?;
        }
        if let Some(v) = self.switching.as_ref() {
            struct_ser.serialize_field("switching", v)?;
        }
        if let Some(v) = self.through_pair.as_ref() {
            struct_ser.serialize_field("throughPair", v)?;
        }
        if let Some(v) = self.cable_supply.as_ref() {
            struct_ser.serialize_field("cableSupply", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PinNameVocab {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "supply",
            "gate",
            "source",
            "drain",
            "exposed_pad",
            "exposedPad",
            "anode",
            "cathode",
            "feedback",
            "switching",
            "through_pair",
            "throughPair",
            "cable_supply",
            "cableSupply",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Supply,
            Gate,
            Source,
            Drain,
            ExposedPad,
            Anode,
            Cathode,
            Feedback,
            Switching,
            ThroughPair,
            CableSupply,
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
                            "supply" => Ok(GeneratedField::Supply),
                            "gate" => Ok(GeneratedField::Gate),
                            "source" => Ok(GeneratedField::Source),
                            "drain" => Ok(GeneratedField::Drain),
                            "exposedPad" | "exposed_pad" => Ok(GeneratedField::ExposedPad),
                            "anode" => Ok(GeneratedField::Anode),
                            "cathode" => Ok(GeneratedField::Cathode),
                            "feedback" => Ok(GeneratedField::Feedback),
                            "switching" => Ok(GeneratedField::Switching),
                            "throughPair" | "through_pair" => Ok(GeneratedField::ThroughPair),
                            "cableSupply" | "cable_supply" => Ok(GeneratedField::CableSupply),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PinNameVocab;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.PinNameVocab")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PinNameVocab, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut supply__ = None;
                let mut gate__ = None;
                let mut source__ = None;
                let mut drain__ = None;
                let mut exposed_pad__ = None;
                let mut anode__ = None;
                let mut cathode__ = None;
                let mut feedback__ = None;
                let mut switching__ = None;
                let mut through_pair__ = None;
                let mut cable_supply__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Supply => {
                            if supply__.is_some() {
                                return Err(serde::de::Error::duplicate_field("supply"));
                            }
                            supply__ = map_.next_value()?;
                        }
                        GeneratedField::Gate => {
                            if gate__.is_some() {
                                return Err(serde::de::Error::duplicate_field("gate"));
                            }
                            gate__ = map_.next_value()?;
                        }
                        GeneratedField::Source => {
                            if source__.is_some() {
                                return Err(serde::de::Error::duplicate_field("source"));
                            }
                            source__ = map_.next_value()?;
                        }
                        GeneratedField::Drain => {
                            if drain__.is_some() {
                                return Err(serde::de::Error::duplicate_field("drain"));
                            }
                            drain__ = map_.next_value()?;
                        }
                        GeneratedField::ExposedPad => {
                            if exposed_pad__.is_some() {
                                return Err(serde::de::Error::duplicate_field("exposedPad"));
                            }
                            exposed_pad__ = map_.next_value()?;
                        }
                        GeneratedField::Anode => {
                            if anode__.is_some() {
                                return Err(serde::de::Error::duplicate_field("anode"));
                            }
                            anode__ = map_.next_value()?;
                        }
                        GeneratedField::Cathode => {
                            if cathode__.is_some() {
                                return Err(serde::de::Error::duplicate_field("cathode"));
                            }
                            cathode__ = map_.next_value()?;
                        }
                        GeneratedField::Feedback => {
                            if feedback__.is_some() {
                                return Err(serde::de::Error::duplicate_field("feedback"));
                            }
                            feedback__ = map_.next_value()?;
                        }
                        GeneratedField::Switching => {
                            if switching__.is_some() {
                                return Err(serde::de::Error::duplicate_field("switching"));
                            }
                            switching__ = map_.next_value()?;
                        }
                        GeneratedField::ThroughPair => {
                            if through_pair__.is_some() {
                                return Err(serde::de::Error::duplicate_field("throughPair"));
                            }
                            through_pair__ = map_.next_value()?;
                        }
                        GeneratedField::CableSupply => {
                            if cable_supply__.is_some() {
                                return Err(serde::de::Error::duplicate_field("cableSupply"));
                            }
                            cable_supply__ = map_.next_value()?;
                        }
                    }
                }
                Ok(PinNameVocab {
                    supply: supply__,
                    gate: gate__,
                    source: source__,
                    drain: drain__,
                    exposed_pad: exposed_pad__,
                    anode: anode__,
                    cathode: cathode__,
                    feedback: feedback__,
                    switching: switching__,
                    through_pair: through_pair__,
                    cable_supply: cable_supply__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.PinNameVocab", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PinPair {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.a.is_empty() {
            len += 1;
        }
        if !self.b.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.PinPair", len)?;
        if !self.a.is_empty() {
            struct_ser.serialize_field("a", &self.a)?;
        }
        if !self.b.is_empty() {
            struct_ser.serialize_field("b", &self.b)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PinPair {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "a",
            "b",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            A,
            B,
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
                            "a" => Ok(GeneratedField::A),
                            "b" => Ok(GeneratedField::B),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PinPair;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.PinPair")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PinPair, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut a__ = None;
                let mut b__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::A => {
                            if a__.is_some() {
                                return Err(serde::de::Error::duplicate_field("a"));
                            }
                            a__ = Some(map_.next_value()?);
                        }
                        GeneratedField::B => {
                            if b__.is_some() {
                                return Err(serde::de::Error::duplicate_field("b"));
                            }
                            b__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(PinPair {
                    a: a__.unwrap_or_default(),
                    b: b__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.PinPair", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PinPairVocab {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.pairs.is_empty() {
            len += 1;
        }
        if self.replace {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.PinPairVocab", len)?;
        if !self.pairs.is_empty() {
            struct_ser.serialize_field("pairs", &self.pairs)?;
        }
        if self.replace {
            struct_ser.serialize_field("replace", &self.replace)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PinPairVocab {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "pairs",
            "replace",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Pairs,
            Replace,
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
                            "pairs" => Ok(GeneratedField::Pairs),
                            "replace" => Ok(GeneratedField::Replace),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PinPairVocab;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.PinPairVocab")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PinPairVocab, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut pairs__ = None;
                let mut replace__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Pairs => {
                            if pairs__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pairs"));
                            }
                            pairs__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Replace => {
                            if replace__.is_some() {
                                return Err(serde::de::Error::duplicate_field("replace"));
                            }
                            replace__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(PinPairVocab {
                    pairs: pairs__.unwrap_or_default(),
                    replace: replace__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.PinPairVocab", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PowerSequence {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.name.is_empty() {
            len += 1;
        }
        if !self.relation.is_empty() {
            len += 1;
        }
        if !self.order.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.PowerSequence", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.relation.is_empty() {
            struct_ser.serialize_field("relation", &self.relation)?;
        }
        if !self.order.is_empty() {
            struct_ser.serialize_field("order", &self.order)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PowerSequence {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "relation",
            "order",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Relation,
            Order,
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
                            "name" => Ok(GeneratedField::Name),
                            "relation" => Ok(GeneratedField::Relation),
                            "order" => Ok(GeneratedField::Order),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PowerSequence;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.PowerSequence")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PowerSequence, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut relation__ = None;
                let mut order__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Relation => {
                            if relation__.is_some() {
                                return Err(serde::de::Error::duplicate_field("relation"));
                            }
                            relation__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Order => {
                            if order__.is_some() {
                                return Err(serde::de::Error::duplicate_field("order"));
                            }
                            order__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(PowerSequence {
                    name: name__.unwrap_or_default(),
                    relation: relation__.unwrap_or_default(),
                    order: order__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.PowerSequence", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SequenceStage {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.rail.is_empty() {
            len += 1;
        }
        if !self.good.is_empty() {
            len += 1;
        }
        if !self.enable.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.SequenceStage", len)?;
        if !self.rail.is_empty() {
            struct_ser.serialize_field("rail", &self.rail)?;
        }
        if !self.good.is_empty() {
            struct_ser.serialize_field("good", &self.good)?;
        }
        if !self.enable.is_empty() {
            struct_ser.serialize_field("enable", &self.enable)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SequenceStage {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "rail",
            "good",
            "enable",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Rail,
            Good,
            Enable,
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
                            "rail" => Ok(GeneratedField::Rail),
                            "good" => Ok(GeneratedField::Good),
                            "enable" => Ok(GeneratedField::Enable),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SequenceStage;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.SequenceStage")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SequenceStage, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut rail__ = None;
                let mut good__ = None;
                let mut enable__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Rail => {
                            if rail__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rail"));
                            }
                            rail__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Good => {
                            if good__.is_some() {
                                return Err(serde::de::Error::duplicate_field("good"));
                            }
                            good__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Enable => {
                            if enable__.is_some() {
                                return Err(serde::de::Error::duplicate_field("enable"));
                            }
                            enable__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(SequenceStage {
                    rail: rail__.unwrap_or_default(),
                    good: good__.unwrap_or_default(),
                    enable: enable__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.SequenceStage", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for StrapGroup {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.name.is_empty() {
            len += 1;
        }
        if !self.device.is_empty() {
            len += 1;
        }
        if !self.nets.is_empty() {
            len += 1;
        }
        if self.value != 0 {
            len += 1;
        }
        if !self.bus.is_empty() {
            len += 1;
        }
        if !self.default.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.StrapGroup", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.device.is_empty() {
            struct_ser.serialize_field("device", &self.device)?;
        }
        if !self.nets.is_empty() {
            struct_ser.serialize_field("nets", &self.nets)?;
        }
        if self.value != 0 {
            struct_ser.serialize_field("value", &self.value)?;
        }
        if !self.bus.is_empty() {
            struct_ser.serialize_field("bus", &self.bus)?;
        }
        if !self.default.is_empty() {
            struct_ser.serialize_field("default", &self.default)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for StrapGroup {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "device",
            "nets",
            "value",
            "bus",
            "default",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Device,
            Nets,
            Value,
            Bus,
            Default,
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
                            "name" => Ok(GeneratedField::Name),
                            "device" => Ok(GeneratedField::Device),
                            "nets" => Ok(GeneratedField::Nets),
                            "value" => Ok(GeneratedField::Value),
                            "bus" => Ok(GeneratedField::Bus),
                            "default" => Ok(GeneratedField::Default),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = StrapGroup;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.StrapGroup")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<StrapGroup, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut device__ = None;
                let mut nets__ = None;
                let mut value__ = None;
                let mut bus__ = None;
                let mut default__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Device => {
                            if device__.is_some() {
                                return Err(serde::de::Error::duplicate_field("device"));
                            }
                            device__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Nets => {
                            if nets__.is_some() {
                                return Err(serde::de::Error::duplicate_field("nets"));
                            }
                            nets__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Value => {
                            if value__.is_some() {
                                return Err(serde::de::Error::duplicate_field("value"));
                            }
                            value__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Bus => {
                            if bus__.is_some() {
                                return Err(serde::de::Error::duplicate_field("bus"));
                            }
                            bus__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Default => {
                            if default__.is_some() {
                                return Err(serde::de::Error::duplicate_field("default"));
                            }
                            default__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(StrapGroup {
                    name: name__.unwrap_or_default(),
                    device: device__.unwrap_or_default(),
                    nets: nets__.unwrap_or_default(),
                    value: value__.unwrap_or_default(),
                    bus: bus__.unwrap_or_default(),
                    default: default__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.StrapGroup", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for VocabPatterns {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.patterns.is_empty() {
            len += 1;
        }
        if self.replace {
            len += 1;
        }
        if !self.exclude.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.config.VocabPatterns", len)?;
        if !self.patterns.is_empty() {
            struct_ser.serialize_field("patterns", &self.patterns)?;
        }
        if self.replace {
            struct_ser.serialize_field("replace", &self.replace)?;
        }
        if !self.exclude.is_empty() {
            struct_ser.serialize_field("exclude", &self.exclude)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for VocabPatterns {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "patterns",
            "replace",
            "exclude",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Patterns,
            Replace,
            Exclude,
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
                            "patterns" => Ok(GeneratedField::Patterns),
                            "replace" => Ok(GeneratedField::Replace),
                            "exclude" => Ok(GeneratedField::Exclude),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = VocabPatterns;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.config.VocabPatterns")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<VocabPatterns, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut patterns__ = None;
                let mut replace__ = None;
                let mut exclude__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Patterns => {
                            if patterns__.is_some() {
                                return Err(serde::de::Error::duplicate_field("patterns"));
                            }
                            patterns__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Replace => {
                            if replace__.is_some() {
                                return Err(serde::de::Error::duplicate_field("replace"));
                            }
                            replace__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Exclude => {
                            if exclude__.is_some() {
                                return Err(serde::de::Error::duplicate_field("exclude"));
                            }
                            exclude__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(VocabPatterns {
                    patterns: patterns__.unwrap_or_default(),
                    replace: replace__.unwrap_or_default(),
                    exclude: exclude__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.config.VocabPatterns", FIELDS, GeneratedVisitor)
    }
}
