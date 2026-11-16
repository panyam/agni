// @generated
impl serde::Serialize for BatchGetPartSpecsRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.mpns.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.BatchGetPartSpecsRequest", len)?;
        if !self.mpns.is_empty() {
            struct_ser.serialize_field("mpns", &self.mpns)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BatchGetPartSpecsRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "mpns",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Mpns,
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
                            "mpns" => Ok(GeneratedField::Mpns),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BatchGetPartSpecsRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.BatchGetPartSpecsRequest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BatchGetPartSpecsRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut mpns__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Mpns => {
                            if mpns__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mpns"));
                            }
                            mpns__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(BatchGetPartSpecsRequest {
                    mpns: mpns__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.BatchGetPartSpecsRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BatchGetPartSpecsResponse {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.specs.is_empty() {
            len += 1;
        }
        if self.generation != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.BatchGetPartSpecsResponse", len)?;
        if !self.specs.is_empty() {
            struct_ser.serialize_field("specs", &self.specs)?;
        }
        if self.generation != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("generation", ToString::to_string(&self.generation).as_str())?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BatchGetPartSpecsResponse {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "specs",
            "generation",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Specs,
            Generation,
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
                            "specs" => Ok(GeneratedField::Specs),
                            "generation" => Ok(GeneratedField::Generation),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BatchGetPartSpecsResponse;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.BatchGetPartSpecsResponse")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BatchGetPartSpecsResponse, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut specs__ = None;
                let mut generation__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Specs => {
                            if specs__.is_some() {
                                return Err(serde::de::Error::duplicate_field("specs"));
                            }
                            specs__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Generation => {
                            if generation__.is_some() {
                                return Err(serde::de::Error::duplicate_field("generation"));
                            }
                            generation__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(BatchGetPartSpecsResponse {
                    specs: specs__.unwrap_or_default(),
                    generation: generation__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.BatchGetPartSpecsResponse", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Condition {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.symbol.is_empty() {
            len += 1;
        }
        if self.eq.is_some() {
            len += 1;
        }
        if self.min.is_some() {
            len += 1;
        }
        if self.max.is_some() {
            len += 1;
        }
        if !self.unit.is_empty() {
            len += 1;
        }
        if !self.raw.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.Condition", len)?;
        if !self.symbol.is_empty() {
            struct_ser.serialize_field("symbol", &self.symbol)?;
        }
        if let Some(v) = self.eq.as_ref() {
            struct_ser.serialize_field("eq", v)?;
        }
        if let Some(v) = self.min.as_ref() {
            struct_ser.serialize_field("min", v)?;
        }
        if let Some(v) = self.max.as_ref() {
            struct_ser.serialize_field("max", v)?;
        }
        if !self.unit.is_empty() {
            struct_ser.serialize_field("unit", &self.unit)?;
        }
        if !self.raw.is_empty() {
            struct_ser.serialize_field("raw", &self.raw)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Condition {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "symbol",
            "eq",
            "min",
            "max",
            "unit",
            "raw",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Symbol,
            Eq,
            Min,
            Max,
            Unit,
            Raw,
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
                            "symbol" => Ok(GeneratedField::Symbol),
                            "eq" => Ok(GeneratedField::Eq),
                            "min" => Ok(GeneratedField::Min),
                            "max" => Ok(GeneratedField::Max),
                            "unit" => Ok(GeneratedField::Unit),
                            "raw" => Ok(GeneratedField::Raw),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Condition;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.Condition")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Condition, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut symbol__ = None;
                let mut eq__ = None;
                let mut min__ = None;
                let mut max__ = None;
                let mut unit__ = None;
                let mut raw__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Symbol => {
                            if symbol__.is_some() {
                                return Err(serde::de::Error::duplicate_field("symbol"));
                            }
                            symbol__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Eq => {
                            if eq__.is_some() {
                                return Err(serde::de::Error::duplicate_field("eq"));
                            }
                            eq__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                        GeneratedField::Min => {
                            if min__.is_some() {
                                return Err(serde::de::Error::duplicate_field("min"));
                            }
                            min__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                        GeneratedField::Max => {
                            if max__.is_some() {
                                return Err(serde::de::Error::duplicate_field("max"));
                            }
                            max__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                        GeneratedField::Unit => {
                            if unit__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unit"));
                            }
                            unit__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Raw => {
                            if raw__.is_some() {
                                return Err(serde::de::Error::duplicate_field("raw"));
                            }
                            raw__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Condition {
                    symbol: symbol__.unwrap_or_default(),
                    eq: eq__,
                    min: min__,
                    max: max__,
                    unit: unit__.unwrap_or_default(),
                    raw: raw__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.Condition", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ConditionCoverage {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "CONDITION_COVERAGE_UNSPECIFIED",
            Self::Complete => "CONDITION_COVERAGE_COMPLETE",
            Self::Partial => "CONDITION_COVERAGE_PARTIAL",
            Self::Unconditional => "CONDITION_COVERAGE_UNCONDITIONAL",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for ConditionCoverage {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "CONDITION_COVERAGE_UNSPECIFIED",
            "CONDITION_COVERAGE_COMPLETE",
            "CONDITION_COVERAGE_PARTIAL",
            "CONDITION_COVERAGE_UNCONDITIONAL",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ConditionCoverage;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                write!(formatter, "expected one of: {:?}", &FIELDS)
            }

            fn visit_i64<E>(self, v: i64) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                i32::try_from(v)
                    .ok()
                    .and_then(|x| x.try_into().ok())
                    .ok_or_else(|| {
                        serde::de::Error::invalid_value(serde::de::Unexpected::Signed(v), &self)
                    })
            }

            fn visit_u64<E>(self, v: u64) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                i32::try_from(v)
                    .ok()
                    .and_then(|x| x.try_into().ok())
                    .ok_or_else(|| {
                        serde::de::Error::invalid_value(serde::de::Unexpected::Unsigned(v), &self)
                    })
            }

            fn visit_str<E>(self, value: &str) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                match value {
                    "CONDITION_COVERAGE_UNSPECIFIED" => Ok(ConditionCoverage::Unspecified),
                    "CONDITION_COVERAGE_COMPLETE" => Ok(ConditionCoverage::Complete),
                    "CONDITION_COVERAGE_PARTIAL" => Ok(ConditionCoverage::Partial),
                    "CONDITION_COVERAGE_UNCONDITIONAL" => Ok(ConditionCoverage::Unconditional),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for GetGenerationRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let len = 0;
        let struct_ser = serializer.serialize_struct("agni.v1.param.GetGenerationRequest", len)?;
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for GetGenerationRequest {
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
            type Value = GetGenerationRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.GetGenerationRequest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<GetGenerationRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                while map_.next_key::<GeneratedField>()?.is_some() {
                    let _ = map_.next_value::<serde::de::IgnoredAny>()?;
                }
                Ok(GetGenerationRequest {
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.GetGenerationRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for GetGenerationResponse {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.generation != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.GetGenerationResponse", len)?;
        if self.generation != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("generation", ToString::to_string(&self.generation).as_str())?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for GetGenerationResponse {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "generation",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Generation,
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
                            "generation" => Ok(GeneratedField::Generation),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = GetGenerationResponse;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.GetGenerationResponse")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<GetGenerationResponse, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut generation__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Generation => {
                            if generation__.is_some() {
                                return Err(serde::de::Error::duplicate_field("generation"));
                            }
                            generation__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(GetGenerationResponse {
                    generation: generation__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.GetGenerationResponse", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for LimitKind {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "LIMIT_KIND_UNSPECIFIED",
            Self::AbsoluteMax => "LIMIT_KIND_ABSOLUTE_MAX",
            Self::RecommendedOperating => "LIMIT_KIND_RECOMMENDED_OPERATING",
            Self::Characteristic => "LIMIT_KIND_CHARACTERISTIC",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for LimitKind {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "LIMIT_KIND_UNSPECIFIED",
            "LIMIT_KIND_ABSOLUTE_MAX",
            "LIMIT_KIND_RECOMMENDED_OPERATING",
            "LIMIT_KIND_CHARACTERISTIC",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = LimitKind;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                write!(formatter, "expected one of: {:?}", &FIELDS)
            }

            fn visit_i64<E>(self, v: i64) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                i32::try_from(v)
                    .ok()
                    .and_then(|x| x.try_into().ok())
                    .ok_or_else(|| {
                        serde::de::Error::invalid_value(serde::de::Unexpected::Signed(v), &self)
                    })
            }

            fn visit_u64<E>(self, v: u64) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                i32::try_from(v)
                    .ok()
                    .and_then(|x| x.try_into().ok())
                    .ok_or_else(|| {
                        serde::de::Error::invalid_value(serde::de::Unexpected::Unsigned(v), &self)
                    })
            }

            fn visit_str<E>(self, value: &str) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                match value {
                    "LIMIT_KIND_UNSPECIFIED" => Ok(LimitKind::Unspecified),
                    "LIMIT_KIND_ABSOLUTE_MAX" => Ok(LimitKind::AbsoluteMax),
                    "LIMIT_KIND_RECOMMENDED_OPERATING" => Ok(LimitKind::RecommendedOperating),
                    "LIMIT_KIND_CHARACTERISTIC" => Ok(LimitKind::Characteristic),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for Modality {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "MODALITY_UNSPECIFIED",
            Self::Required => "MODALITY_REQUIRED",
            Self::Recommended => "MODALITY_RECOMMENDED",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for Modality {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "MODALITY_UNSPECIFIED",
            "MODALITY_REQUIRED",
            "MODALITY_RECOMMENDED",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Modality;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                write!(formatter, "expected one of: {:?}", &FIELDS)
            }

            fn visit_i64<E>(self, v: i64) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                i32::try_from(v)
                    .ok()
                    .and_then(|x| x.try_into().ok())
                    .ok_or_else(|| {
                        serde::de::Error::invalid_value(serde::de::Unexpected::Signed(v), &self)
                    })
            }

            fn visit_u64<E>(self, v: u64) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                i32::try_from(v)
                    .ok()
                    .and_then(|x| x.try_into().ok())
                    .ok_or_else(|| {
                        serde::de::Error::invalid_value(serde::de::Unexpected::Unsigned(v), &self)
                    })
            }

            fn visit_str<E>(self, value: &str) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                match value {
                    "MODALITY_UNSPECIFIED" => Ok(Modality::Unspecified),
                    "MODALITY_REQUIRED" => Ok(Modality::Required),
                    "MODALITY_RECOMMENDED" => Ok(Modality::Recommended),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for Package {
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
        if !self.name.is_empty() {
            len += 1;
        }
        if !self.mpn_suffix.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.Package", len)?;
        if !self.id.is_empty() {
            struct_ser.serialize_field("id", &self.id)?;
        }
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.mpn_suffix.is_empty() {
            struct_ser.serialize_field("mpnSuffix", &self.mpn_suffix)?;
        }
        if !self.attributes.is_empty() {
            struct_ser.serialize_field("attributes", &self.attributes)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Package {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "id",
            "name",
            "mpn_suffix",
            "mpnSuffix",
            "attributes",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Id,
            Name,
            MpnSuffix,
            Attributes,
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
                            "name" => Ok(GeneratedField::Name),
                            "mpnSuffix" | "mpn_suffix" => Ok(GeneratedField::MpnSuffix),
                            "attributes" => Ok(GeneratedField::Attributes),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Package;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.Package")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Package, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut id__ = None;
                let mut name__ = None;
                let mut mpn_suffix__ = None;
                let mut attributes__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Id => {
                            if id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("id"));
                            }
                            id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::MpnSuffix => {
                            if mpn_suffix__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mpnSuffix"));
                            }
                            mpn_suffix__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Attributes => {
                            if attributes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("attributes"));
                            }
                            attributes__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                    }
                }
                Ok(Package {
                    id: id__.unwrap_or_default(),
                    name: name__.unwrap_or_default(),
                    mpn_suffix: mpn_suffix__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.Package", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ParamProvenance {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.doc_ref.is_empty() {
            len += 1;
        }
        if self.page != 0 {
            len += 1;
        }
        if !self.table_or_figure.is_empty() {
            len += 1;
        }
        if !self.method.is_empty() {
            len += 1;
        }
        if self.confidence != 0. {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.ParamProvenance", len)?;
        if !self.doc_ref.is_empty() {
            struct_ser.serialize_field("docRef", &self.doc_ref)?;
        }
        if self.page != 0 {
            struct_ser.serialize_field("page", &self.page)?;
        }
        if !self.table_or_figure.is_empty() {
            struct_ser.serialize_field("tableOrFigure", &self.table_or_figure)?;
        }
        if !self.method.is_empty() {
            struct_ser.serialize_field("method", &self.method)?;
        }
        if self.confidence != 0. {
            struct_ser.serialize_field("confidence", &self.confidence)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ParamProvenance {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "doc_ref",
            "docRef",
            "page",
            "table_or_figure",
            "tableOrFigure",
            "method",
            "confidence",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            DocRef,
            Page,
            TableOrFigure,
            Method,
            Confidence,
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
                            "docRef" | "doc_ref" => Ok(GeneratedField::DocRef),
                            "page" => Ok(GeneratedField::Page),
                            "tableOrFigure" | "table_or_figure" => Ok(GeneratedField::TableOrFigure),
                            "method" => Ok(GeneratedField::Method),
                            "confidence" => Ok(GeneratedField::Confidence),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ParamProvenance;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.ParamProvenance")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ParamProvenance, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut doc_ref__ = None;
                let mut page__ = None;
                let mut table_or_figure__ = None;
                let mut method__ = None;
                let mut confidence__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::DocRef => {
                            if doc_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("docRef"));
                            }
                            doc_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Page => {
                            if page__.is_some() {
                                return Err(serde::de::Error::duplicate_field("page"));
                            }
                            page__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::TableOrFigure => {
                            if table_or_figure__.is_some() {
                                return Err(serde::de::Error::duplicate_field("tableOrFigure"));
                            }
                            table_or_figure__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Method => {
                            if method__.is_some() {
                                return Err(serde::de::Error::duplicate_field("method"));
                            }
                            method__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Confidence => {
                            if confidence__.is_some() {
                                return Err(serde::de::Error::duplicate_field("confidence"));
                            }
                            confidence__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(ParamProvenance {
                    doc_ref: doc_ref__.unwrap_or_default(),
                    page: page__.unwrap_or_default(),
                    table_or_figure: table_or_figure__.unwrap_or_default(),
                    method: method__.unwrap_or_default(),
                    confidence: confidence__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.ParamProvenance", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Parameter {
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
        if !self.symbol.is_empty() {
            len += 1;
        }
        if !self.canonical_id.is_empty() {
            len += 1;
        }
        if self.limit_kind != 0 {
            len += 1;
        }
        if self.value.is_some() {
            len += 1;
        }
        if !self.unit.is_empty() {
            len += 1;
        }
        if !self.conditions.is_empty() {
            len += 1;
        }
        if self.condition_coverage != 0 {
            len += 1;
        }
        if !self.applies_to.is_empty() {
            len += 1;
        }
        if !self.pin_refs.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        if self.verification.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.Parameter", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.symbol.is_empty() {
            struct_ser.serialize_field("symbol", &self.symbol)?;
        }
        if !self.canonical_id.is_empty() {
            struct_ser.serialize_field("canonicalId", &self.canonical_id)?;
        }
        if self.limit_kind != 0 {
            let v = LimitKind::try_from(self.limit_kind)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.limit_kind)))?;
            struct_ser.serialize_field("limitKind", &v)?;
        }
        if let Some(v) = self.value.as_ref() {
            struct_ser.serialize_field("value", v)?;
        }
        if !self.unit.is_empty() {
            struct_ser.serialize_field("unit", &self.unit)?;
        }
        if !self.conditions.is_empty() {
            struct_ser.serialize_field("conditions", &self.conditions)?;
        }
        if self.condition_coverage != 0 {
            let v = ConditionCoverage::try_from(self.condition_coverage)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.condition_coverage)))?;
            struct_ser.serialize_field("conditionCoverage", &v)?;
        }
        if !self.applies_to.is_empty() {
            struct_ser.serialize_field("appliesTo", &self.applies_to)?;
        }
        if !self.pin_refs.is_empty() {
            struct_ser.serialize_field("pinRefs", &self.pin_refs)?;
        }
        if !self.attributes.is_empty() {
            struct_ser.serialize_field("attributes", &self.attributes)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        if let Some(v) = self.verification.as_ref() {
            struct_ser.serialize_field("verification", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Parameter {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "symbol",
            "canonical_id",
            "canonicalId",
            "limit_kind",
            "limitKind",
            "value",
            "unit",
            "conditions",
            "condition_coverage",
            "conditionCoverage",
            "applies_to",
            "appliesTo",
            "pin_refs",
            "pinRefs",
            "attributes",
            "prov",
            "verification",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Symbol,
            CanonicalId,
            LimitKind,
            Value,
            Unit,
            Conditions,
            ConditionCoverage,
            AppliesTo,
            PinRefs,
            Attributes,
            Prov,
            Verification,
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
                            "symbol" => Ok(GeneratedField::Symbol),
                            "canonicalId" | "canonical_id" => Ok(GeneratedField::CanonicalId),
                            "limitKind" | "limit_kind" => Ok(GeneratedField::LimitKind),
                            "value" => Ok(GeneratedField::Value),
                            "unit" => Ok(GeneratedField::Unit),
                            "conditions" => Ok(GeneratedField::Conditions),
                            "conditionCoverage" | "condition_coverage" => Ok(GeneratedField::ConditionCoverage),
                            "appliesTo" | "applies_to" => Ok(GeneratedField::AppliesTo),
                            "pinRefs" | "pin_refs" => Ok(GeneratedField::PinRefs),
                            "attributes" => Ok(GeneratedField::Attributes),
                            "prov" => Ok(GeneratedField::Prov),
                            "verification" => Ok(GeneratedField::Verification),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Parameter;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.Parameter")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Parameter, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut symbol__ = None;
                let mut canonical_id__ = None;
                let mut limit_kind__ = None;
                let mut value__ = None;
                let mut unit__ = None;
                let mut conditions__ = None;
                let mut condition_coverage__ = None;
                let mut applies_to__ = None;
                let mut pin_refs__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                let mut verification__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Symbol => {
                            if symbol__.is_some() {
                                return Err(serde::de::Error::duplicate_field("symbol"));
                            }
                            symbol__ = Some(map_.next_value()?);
                        }
                        GeneratedField::CanonicalId => {
                            if canonical_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("canonicalId"));
                            }
                            canonical_id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::LimitKind => {
                            if limit_kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("limitKind"));
                            }
                            limit_kind__ = Some(map_.next_value::<LimitKind>()? as i32);
                        }
                        GeneratedField::Value => {
                            if value__.is_some() {
                                return Err(serde::de::Error::duplicate_field("value"));
                            }
                            value__ = map_.next_value()?;
                        }
                        GeneratedField::Unit => {
                            if unit__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unit"));
                            }
                            unit__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Conditions => {
                            if conditions__.is_some() {
                                return Err(serde::de::Error::duplicate_field("conditions"));
                            }
                            conditions__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ConditionCoverage => {
                            if condition_coverage__.is_some() {
                                return Err(serde::de::Error::duplicate_field("conditionCoverage"));
                            }
                            condition_coverage__ = Some(map_.next_value::<ConditionCoverage>()? as i32);
                        }
                        GeneratedField::AppliesTo => {
                            if applies_to__.is_some() {
                                return Err(serde::de::Error::duplicate_field("appliesTo"));
                            }
                            applies_to__ = Some(map_.next_value()?);
                        }
                        GeneratedField::PinRefs => {
                            if pin_refs__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pinRefs"));
                            }
                            pin_refs__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Attributes => {
                            if attributes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("attributes"));
                            }
                            attributes__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                        GeneratedField::Verification => {
                            if verification__.is_some() {
                                return Err(serde::de::Error::duplicate_field("verification"));
                            }
                            verification__ = map_.next_value()?;
                        }
                    }
                }
                Ok(Parameter {
                    name: name__.unwrap_or_default(),
                    symbol: symbol__.unwrap_or_default(),
                    canonical_id: canonical_id__.unwrap_or_default(),
                    limit_kind: limit_kind__.unwrap_or_default(),
                    value: value__,
                    unit: unit__.unwrap_or_default(),
                    conditions: conditions__.unwrap_or_default(),
                    condition_coverage: condition_coverage__.unwrap_or_default(),
                    applies_to: applies_to__.unwrap_or_default(),
                    pin_refs: pin_refs__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                    verification: verification__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.Parameter", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PartSpec {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.mpn.is_empty() {
            len += 1;
        }
        if !self.manufacturer.is_empty() {
            len += 1;
        }
        if !self.device_class.is_empty() {
            len += 1;
        }
        if !self.docs.is_empty() {
            len += 1;
        }
        if !self.parameters.is_empty() {
            len += 1;
        }
        if !self.packages.is_empty() {
            len += 1;
        }
        if !self.pins.is_empty() {
            len += 1;
        }
        if !self.relations.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.PartSpec", len)?;
        if !self.mpn.is_empty() {
            struct_ser.serialize_field("mpn", &self.mpn)?;
        }
        if !self.manufacturer.is_empty() {
            struct_ser.serialize_field("manufacturer", &self.manufacturer)?;
        }
        if !self.device_class.is_empty() {
            struct_ser.serialize_field("deviceClass", &self.device_class)?;
        }
        if !self.docs.is_empty() {
            struct_ser.serialize_field("docs", &self.docs)?;
        }
        if !self.parameters.is_empty() {
            struct_ser.serialize_field("parameters", &self.parameters)?;
        }
        if !self.packages.is_empty() {
            struct_ser.serialize_field("packages", &self.packages)?;
        }
        if !self.pins.is_empty() {
            struct_ser.serialize_field("pins", &self.pins)?;
        }
        if !self.relations.is_empty() {
            struct_ser.serialize_field("relations", &self.relations)?;
        }
        if !self.attributes.is_empty() {
            struct_ser.serialize_field("attributes", &self.attributes)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PartSpec {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "mpn",
            "manufacturer",
            "device_class",
            "deviceClass",
            "docs",
            "parameters",
            "packages",
            "pins",
            "relations",
            "attributes",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Mpn,
            Manufacturer,
            DeviceClass,
            Docs,
            Parameters,
            Packages,
            Pins,
            Relations,
            Attributes,
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
                            "manufacturer" => Ok(GeneratedField::Manufacturer),
                            "deviceClass" | "device_class" => Ok(GeneratedField::DeviceClass),
                            "docs" => Ok(GeneratedField::Docs),
                            "parameters" => Ok(GeneratedField::Parameters),
                            "packages" => Ok(GeneratedField::Packages),
                            "pins" => Ok(GeneratedField::Pins),
                            "relations" => Ok(GeneratedField::Relations),
                            "attributes" => Ok(GeneratedField::Attributes),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PartSpec;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.PartSpec")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PartSpec, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut mpn__ = None;
                let mut manufacturer__ = None;
                let mut device_class__ = None;
                let mut docs__ = None;
                let mut parameters__ = None;
                let mut packages__ = None;
                let mut pins__ = None;
                let mut relations__ = None;
                let mut attributes__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Mpn => {
                            if mpn__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mpn"));
                            }
                            mpn__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Manufacturer => {
                            if manufacturer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("manufacturer"));
                            }
                            manufacturer__ = Some(map_.next_value()?);
                        }
                        GeneratedField::DeviceClass => {
                            if device_class__.is_some() {
                                return Err(serde::de::Error::duplicate_field("deviceClass"));
                            }
                            device_class__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Docs => {
                            if docs__.is_some() {
                                return Err(serde::de::Error::duplicate_field("docs"));
                            }
                            docs__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Parameters => {
                            if parameters__.is_some() {
                                return Err(serde::de::Error::duplicate_field("parameters"));
                            }
                            parameters__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Packages => {
                            if packages__.is_some() {
                                return Err(serde::de::Error::duplicate_field("packages"));
                            }
                            packages__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Pins => {
                            if pins__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pins"));
                            }
                            pins__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Relations => {
                            if relations__.is_some() {
                                return Err(serde::de::Error::duplicate_field("relations"));
                            }
                            relations__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Attributes => {
                            if attributes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("attributes"));
                            }
                            attributes__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                    }
                }
                Ok(PartSpec {
                    mpn: mpn__.unwrap_or_default(),
                    manufacturer: manufacturer__.unwrap_or_default(),
                    device_class: device_class__.unwrap_or_default(),
                    docs: docs__.unwrap_or_default(),
                    parameters: parameters__.unwrap_or_default(),
                    packages: packages__.unwrap_or_default(),
                    pins: pins__.unwrap_or_default(),
                    relations: relations__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.PartSpec", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Pin {
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
        if !self.name.is_empty() {
            len += 1;
        }
        if self.function != 0 {
            len += 1;
        }
        if !self.description.is_empty() {
            len += 1;
        }
        if !self.numbers.is_empty() {
            len += 1;
        }
        if !self.alternates.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.Pin", len)?;
        if !self.id.is_empty() {
            struct_ser.serialize_field("id", &self.id)?;
        }
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if self.function != 0 {
            let v = PinFunction::try_from(self.function)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.function)))?;
            struct_ser.serialize_field("function", &v)?;
        }
        if !self.description.is_empty() {
            struct_ser.serialize_field("description", &self.description)?;
        }
        if !self.numbers.is_empty() {
            struct_ser.serialize_field("numbers", &self.numbers)?;
        }
        if !self.alternates.is_empty() {
            struct_ser.serialize_field("alternates", &self.alternates)?;
        }
        if !self.attributes.is_empty() {
            struct_ser.serialize_field("attributes", &self.attributes)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Pin {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "id",
            "name",
            "function",
            "description",
            "numbers",
            "alternates",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Id,
            Name,
            Function,
            Description,
            Numbers,
            Alternates,
            Attributes,
            Prov,
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
                            "name" => Ok(GeneratedField::Name),
                            "function" => Ok(GeneratedField::Function),
                            "description" => Ok(GeneratedField::Description),
                            "numbers" => Ok(GeneratedField::Numbers),
                            "alternates" => Ok(GeneratedField::Alternates),
                            "attributes" => Ok(GeneratedField::Attributes),
                            "prov" => Ok(GeneratedField::Prov),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Pin;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.Pin")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Pin, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut id__ = None;
                let mut name__ = None;
                let mut function__ = None;
                let mut description__ = None;
                let mut numbers__ = None;
                let mut alternates__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Id => {
                            if id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("id"));
                            }
                            id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Function => {
                            if function__.is_some() {
                                return Err(serde::de::Error::duplicate_field("function"));
                            }
                            function__ = Some(map_.next_value::<PinFunction>()? as i32);
                        }
                        GeneratedField::Description => {
                            if description__.is_some() {
                                return Err(serde::de::Error::duplicate_field("description"));
                            }
                            description__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Numbers => {
                            if numbers__.is_some() {
                                return Err(serde::de::Error::duplicate_field("numbers"));
                            }
                            numbers__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Alternates => {
                            if alternates__.is_some() {
                                return Err(serde::de::Error::duplicate_field("alternates"));
                            }
                            alternates__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Attributes => {
                            if attributes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("attributes"));
                            }
                            attributes__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(Pin {
                    id: id__.unwrap_or_default(),
                    name: name__.unwrap_or_default(),
                    function: function__.unwrap_or_default(),
                    description: description__.unwrap_or_default(),
                    numbers: numbers__.unwrap_or_default(),
                    alternates: alternates__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.Pin", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PinAlternate {
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
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.PinAlternate", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PinAlternate {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
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
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PinAlternate;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.PinAlternate")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PinAlternate, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(PinAlternate {
                    name: name__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.PinAlternate", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PinFunction {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "PIN_FUNCTION_UNSPECIFIED",
            Self::PowerInput => "PIN_FUNCTION_POWER_INPUT",
            Self::PowerOutput => "PIN_FUNCTION_POWER_OUTPUT",
            Self::Ground => "PIN_FUNCTION_GROUND",
            Self::Input => "PIN_FUNCTION_INPUT",
            Self::Output => "PIN_FUNCTION_OUTPUT",
            Self::Bidirectional => "PIN_FUNCTION_BIDIRECTIONAL",
            Self::Passive => "PIN_FUNCTION_PASSIVE",
            Self::NoConnect => "PIN_FUNCTION_NO_CONNECT",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for PinFunction {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "PIN_FUNCTION_UNSPECIFIED",
            "PIN_FUNCTION_POWER_INPUT",
            "PIN_FUNCTION_POWER_OUTPUT",
            "PIN_FUNCTION_GROUND",
            "PIN_FUNCTION_INPUT",
            "PIN_FUNCTION_OUTPUT",
            "PIN_FUNCTION_BIDIRECTIONAL",
            "PIN_FUNCTION_PASSIVE",
            "PIN_FUNCTION_NO_CONNECT",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PinFunction;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                write!(formatter, "expected one of: {:?}", &FIELDS)
            }

            fn visit_i64<E>(self, v: i64) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                i32::try_from(v)
                    .ok()
                    .and_then(|x| x.try_into().ok())
                    .ok_or_else(|| {
                        serde::de::Error::invalid_value(serde::de::Unexpected::Signed(v), &self)
                    })
            }

            fn visit_u64<E>(self, v: u64) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                i32::try_from(v)
                    .ok()
                    .and_then(|x| x.try_into().ok())
                    .ok_or_else(|| {
                        serde::de::Error::invalid_value(serde::de::Unexpected::Unsigned(v), &self)
                    })
            }

            fn visit_str<E>(self, value: &str) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                match value {
                    "PIN_FUNCTION_UNSPECIFIED" => Ok(PinFunction::Unspecified),
                    "PIN_FUNCTION_POWER_INPUT" => Ok(PinFunction::PowerInput),
                    "PIN_FUNCTION_POWER_OUTPUT" => Ok(PinFunction::PowerOutput),
                    "PIN_FUNCTION_GROUND" => Ok(PinFunction::Ground),
                    "PIN_FUNCTION_INPUT" => Ok(PinFunction::Input),
                    "PIN_FUNCTION_OUTPUT" => Ok(PinFunction::Output),
                    "PIN_FUNCTION_BIDIRECTIONAL" => Ok(PinFunction::Bidirectional),
                    "PIN_FUNCTION_PASSIVE" => Ok(PinFunction::Passive),
                    "PIN_FUNCTION_NO_CONNECT" => Ok(PinFunction::NoConnect),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for PinNumber {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.package_ref.is_empty() {
            len += 1;
        }
        if !self.number.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.PinNumber", len)?;
        if !self.package_ref.is_empty() {
            struct_ser.serialize_field("packageRef", &self.package_ref)?;
        }
        if !self.number.is_empty() {
            struct_ser.serialize_field("number", &self.number)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PinNumber {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "package_ref",
            "packageRef",
            "number",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            PackageRef,
            Number,
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
                            "packageRef" | "package_ref" => Ok(GeneratedField::PackageRef),
                            "number" => Ok(GeneratedField::Number),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PinNumber;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.PinNumber")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PinNumber, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut package_ref__ = None;
                let mut number__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::PackageRef => {
                            if package_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("packageRef"));
                            }
                            package_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Number => {
                            if number__.is_some() {
                                return Err(serde::de::Error::duplicate_field("number"));
                            }
                            number__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(PinNumber {
                    package_ref: package_ref__.unwrap_or_default(),
                    number: number__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.PinNumber", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PinRelation {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.subject_pin_ref.is_empty() {
            len += 1;
        }
        if !self.reference_pin_ref.is_empty() {
            len += 1;
        }
        if self.kind != 0 {
            len += 1;
        }
        if self.difference.is_some() {
            len += 1;
        }
        if !self.unit.is_empty() {
            len += 1;
        }
        if self.modality != 0 {
            len += 1;
        }
        if !self.conditions.is_empty() {
            len += 1;
        }
        if !self.raw.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.PinRelation", len)?;
        if !self.subject_pin_ref.is_empty() {
            struct_ser.serialize_field("subjectPinRef", &self.subject_pin_ref)?;
        }
        if !self.reference_pin_ref.is_empty() {
            struct_ser.serialize_field("referencePinRef", &self.reference_pin_ref)?;
        }
        if self.kind != 0 {
            let v = PinRelationKind::try_from(self.kind)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.kind)))?;
            struct_ser.serialize_field("kind", &v)?;
        }
        if let Some(v) = self.difference.as_ref() {
            struct_ser.serialize_field("difference", v)?;
        }
        if !self.unit.is_empty() {
            struct_ser.serialize_field("unit", &self.unit)?;
        }
        if self.modality != 0 {
            let v = Modality::try_from(self.modality)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.modality)))?;
            struct_ser.serialize_field("modality", &v)?;
        }
        if !self.conditions.is_empty() {
            struct_ser.serialize_field("conditions", &self.conditions)?;
        }
        if !self.raw.is_empty() {
            struct_ser.serialize_field("raw", &self.raw)?;
        }
        if !self.attributes.is_empty() {
            struct_ser.serialize_field("attributes", &self.attributes)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PinRelation {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "subject_pin_ref",
            "subjectPinRef",
            "reference_pin_ref",
            "referencePinRef",
            "kind",
            "difference",
            "unit",
            "modality",
            "conditions",
            "raw",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            SubjectPinRef,
            ReferencePinRef,
            Kind,
            Difference,
            Unit,
            Modality,
            Conditions,
            Raw,
            Attributes,
            Prov,
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
                            "subjectPinRef" | "subject_pin_ref" => Ok(GeneratedField::SubjectPinRef),
                            "referencePinRef" | "reference_pin_ref" => Ok(GeneratedField::ReferencePinRef),
                            "kind" => Ok(GeneratedField::Kind),
                            "difference" => Ok(GeneratedField::Difference),
                            "unit" => Ok(GeneratedField::Unit),
                            "modality" => Ok(GeneratedField::Modality),
                            "conditions" => Ok(GeneratedField::Conditions),
                            "raw" => Ok(GeneratedField::Raw),
                            "attributes" => Ok(GeneratedField::Attributes),
                            "prov" => Ok(GeneratedField::Prov),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PinRelation;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.PinRelation")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PinRelation, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut subject_pin_ref__ = None;
                let mut reference_pin_ref__ = None;
                let mut kind__ = None;
                let mut difference__ = None;
                let mut unit__ = None;
                let mut modality__ = None;
                let mut conditions__ = None;
                let mut raw__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::SubjectPinRef => {
                            if subject_pin_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("subjectPinRef"));
                            }
                            subject_pin_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ReferencePinRef => {
                            if reference_pin_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("referencePinRef"));
                            }
                            reference_pin_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value::<PinRelationKind>()? as i32);
                        }
                        GeneratedField::Difference => {
                            if difference__.is_some() {
                                return Err(serde::de::Error::duplicate_field("difference"));
                            }
                            difference__ = map_.next_value()?;
                        }
                        GeneratedField::Unit => {
                            if unit__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unit"));
                            }
                            unit__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Modality => {
                            if modality__.is_some() {
                                return Err(serde::de::Error::duplicate_field("modality"));
                            }
                            modality__ = Some(map_.next_value::<Modality>()? as i32);
                        }
                        GeneratedField::Conditions => {
                            if conditions__.is_some() {
                                return Err(serde::de::Error::duplicate_field("conditions"));
                            }
                            conditions__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Raw => {
                            if raw__.is_some() {
                                return Err(serde::de::Error::duplicate_field("raw"));
                            }
                            raw__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Attributes => {
                            if attributes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("attributes"));
                            }
                            attributes__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(PinRelation {
                    subject_pin_ref: subject_pin_ref__.unwrap_or_default(),
                    reference_pin_ref: reference_pin_ref__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    difference: difference__,
                    unit: unit__.unwrap_or_default(),
                    modality: modality__.unwrap_or_default(),
                    conditions: conditions__.unwrap_or_default(),
                    raw: raw__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.PinRelation", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PinRelationKind {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "PIN_RELATION_KIND_UNSPECIFIED",
            Self::Tracking => "PIN_RELATION_KIND_TRACKING",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for PinRelationKind {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "PIN_RELATION_KIND_UNSPECIFIED",
            "PIN_RELATION_KIND_TRACKING",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PinRelationKind;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                write!(formatter, "expected one of: {:?}", &FIELDS)
            }

            fn visit_i64<E>(self, v: i64) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                i32::try_from(v)
                    .ok()
                    .and_then(|x| x.try_into().ok())
                    .ok_or_else(|| {
                        serde::de::Error::invalid_value(serde::de::Unexpected::Signed(v), &self)
                    })
            }

            fn visit_u64<E>(self, v: u64) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                i32::try_from(v)
                    .ok()
                    .and_then(|x| x.try_into().ok())
                    .ok_or_else(|| {
                        serde::de::Error::invalid_value(serde::de::Unexpected::Unsigned(v), &self)
                    })
            }

            fn visit_str<E>(self, value: &str) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                match value {
                    "PIN_RELATION_KIND_UNSPECIFIED" => Ok(PinRelationKind::Unspecified),
                    "PIN_RELATION_KIND_TRACKING" => Ok(PinRelationKind::Tracking),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for RangeValue {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.min.is_some() {
            len += 1;
        }
        if self.typ.is_some() {
            len += 1;
        }
        if self.max.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.RangeValue", len)?;
        if let Some(v) = self.min.as_ref() {
            struct_ser.serialize_field("min", v)?;
        }
        if let Some(v) = self.typ.as_ref() {
            struct_ser.serialize_field("typ", v)?;
        }
        if let Some(v) = self.max.as_ref() {
            struct_ser.serialize_field("max", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for RangeValue {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "min",
            "typ",
            "max",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Min,
            Typ,
            Max,
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
                            "min" => Ok(GeneratedField::Min),
                            "typ" => Ok(GeneratedField::Typ),
                            "max" => Ok(GeneratedField::Max),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = RangeValue;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.RangeValue")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<RangeValue, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut min__ = None;
                let mut typ__ = None;
                let mut max__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Min => {
                            if min__.is_some() {
                                return Err(serde::de::Error::duplicate_field("min"));
                            }
                            min__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                        GeneratedField::Typ => {
                            if typ__.is_some() {
                                return Err(serde::de::Error::duplicate_field("typ"));
                            }
                            typ__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                        GeneratedField::Max => {
                            if max__.is_some() {
                                return Err(serde::de::Error::duplicate_field("max"));
                            }
                            max__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                    }
                }
                Ok(RangeValue {
                    min: min__,
                    typ: typ__,
                    max: max__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.RangeValue", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SourceDoc {
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
        if !self.title.is_empty() {
            len += 1;
        }
        if !self.vendor.is_empty() {
            len += 1;
        }
        if !self.locator.is_empty() {
            len += 1;
        }
        if !self.content_hash.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.SourceDoc", len)?;
        if !self.id.is_empty() {
            struct_ser.serialize_field("id", &self.id)?;
        }
        if !self.title.is_empty() {
            struct_ser.serialize_field("title", &self.title)?;
        }
        if !self.vendor.is_empty() {
            struct_ser.serialize_field("vendor", &self.vendor)?;
        }
        if !self.locator.is_empty() {
            struct_ser.serialize_field("locator", &self.locator)?;
        }
        if !self.content_hash.is_empty() {
            struct_ser.serialize_field("contentHash", &self.content_hash)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SourceDoc {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "id",
            "title",
            "vendor",
            "locator",
            "content_hash",
            "contentHash",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Id,
            Title,
            Vendor,
            Locator,
            ContentHash,
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
                            "title" => Ok(GeneratedField::Title),
                            "vendor" => Ok(GeneratedField::Vendor),
                            "locator" => Ok(GeneratedField::Locator),
                            "contentHash" | "content_hash" => Ok(GeneratedField::ContentHash),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SourceDoc;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.SourceDoc")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SourceDoc, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut id__ = None;
                let mut title__ = None;
                let mut vendor__ = None;
                let mut locator__ = None;
                let mut content_hash__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Id => {
                            if id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("id"));
                            }
                            id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Title => {
                            if title__.is_some() {
                                return Err(serde::de::Error::duplicate_field("title"));
                            }
                            title__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Vendor => {
                            if vendor__.is_some() {
                                return Err(serde::de::Error::duplicate_field("vendor"));
                            }
                            vendor__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Locator => {
                            if locator__.is_some() {
                                return Err(serde::de::Error::duplicate_field("locator"));
                            }
                            locator__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ContentHash => {
                            if content_hash__.is_some() {
                                return Err(serde::de::Error::duplicate_field("contentHash"));
                            }
                            content_hash__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(SourceDoc {
                    id: id__.unwrap_or_default(),
                    title: title__.unwrap_or_default(),
                    vendor: vendor__.unwrap_or_default(),
                    locator: locator__.unwrap_or_default(),
                    content_hash: content_hash__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.SourceDoc", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Verification {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.by.is_empty() {
            len += 1;
        }
        if !self.doc_content_hash.is_empty() {
            len += 1;
        }
        if !self.at.is_empty() {
            len += 1;
        }
        if !self.note.is_empty() {
            len += 1;
        }
        if !self.doc_revision.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.param.Verification", len)?;
        if !self.by.is_empty() {
            struct_ser.serialize_field("by", &self.by)?;
        }
        if !self.doc_content_hash.is_empty() {
            struct_ser.serialize_field("docContentHash", &self.doc_content_hash)?;
        }
        if !self.at.is_empty() {
            struct_ser.serialize_field("at", &self.at)?;
        }
        if !self.note.is_empty() {
            struct_ser.serialize_field("note", &self.note)?;
        }
        if !self.doc_revision.is_empty() {
            struct_ser.serialize_field("docRevision", &self.doc_revision)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Verification {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "by",
            "doc_content_hash",
            "docContentHash",
            "at",
            "note",
            "doc_revision",
            "docRevision",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            By,
            DocContentHash,
            At,
            Note,
            DocRevision,
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
                            "by" => Ok(GeneratedField::By),
                            "docContentHash" | "doc_content_hash" => Ok(GeneratedField::DocContentHash),
                            "at" => Ok(GeneratedField::At),
                            "note" => Ok(GeneratedField::Note),
                            "docRevision" | "doc_revision" => Ok(GeneratedField::DocRevision),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Verification;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.param.Verification")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Verification, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut by__ = None;
                let mut doc_content_hash__ = None;
                let mut at__ = None;
                let mut note__ = None;
                let mut doc_revision__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::By => {
                            if by__.is_some() {
                                return Err(serde::de::Error::duplicate_field("by"));
                            }
                            by__ = Some(map_.next_value()?);
                        }
                        GeneratedField::DocContentHash => {
                            if doc_content_hash__.is_some() {
                                return Err(serde::de::Error::duplicate_field("docContentHash"));
                            }
                            doc_content_hash__ = Some(map_.next_value()?);
                        }
                        GeneratedField::At => {
                            if at__.is_some() {
                                return Err(serde::de::Error::duplicate_field("at"));
                            }
                            at__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Note => {
                            if note__.is_some() {
                                return Err(serde::de::Error::duplicate_field("note"));
                            }
                            note__ = Some(map_.next_value()?);
                        }
                        GeneratedField::DocRevision => {
                            if doc_revision__.is_some() {
                                return Err(serde::de::Error::duplicate_field("docRevision"));
                            }
                            doc_revision__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Verification {
                    by: by__.unwrap_or_default(),
                    doc_content_hash: doc_content_hash__.unwrap_or_default(),
                    at: at__.unwrap_or_default(),
                    note: note__.unwrap_or_default(),
                    doc_revision: doc_revision__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.param.Verification", FIELDS, GeneratedVisitor)
    }
}
