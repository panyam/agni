// @generated
impl serde::Serialize for BomLine {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.ref_des.is_empty() {
            len += 1;
        }
        if !self.mpn.is_empty() {
            len += 1;
        }
        if !self.manufacturer.is_empty() {
            len += 1;
        }
        if self.quantity != 0 {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.BomLine", len)?;
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if !self.mpn.is_empty() {
            struct_ser.serialize_field("mpn", &self.mpn)?;
        }
        if !self.manufacturer.is_empty() {
            struct_ser.serialize_field("manufacturer", &self.manufacturer)?;
        }
        if self.quantity != 0 {
            struct_ser.serialize_field("quantity", &self.quantity)?;
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
impl<'de> serde::Deserialize<'de> for BomLine {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "ref_des",
            "refDes",
            "mpn",
            "manufacturer",
            "quantity",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            RefDes,
            Mpn,
            Manufacturer,
            Quantity,
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
                            "refDes" | "ref_des" => Ok(GeneratedField::RefDes),
                            "mpn" => Ok(GeneratedField::Mpn),
                            "manufacturer" => Ok(GeneratedField::Manufacturer),
                            "quantity" => Ok(GeneratedField::Quantity),
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
            type Value = BomLine;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.BomLine")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BomLine, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut ref_des__ = None;
                let mut mpn__ = None;
                let mut manufacturer__ = None;
                let mut quantity__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
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
                        GeneratedField::Quantity => {
                            if quantity__.is_some() {
                                return Err(serde::de::Error::duplicate_field("quantity"));
                            }
                            quantity__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
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
                Ok(BomLine {
                    ref_des: ref_des__.unwrap_or_default(),
                    mpn: mpn__.unwrap_or_default(),
                    manufacturer: manufacturer__.unwrap_or_default(),
                    quantity: quantity__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.BomLine", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BusNotModeled {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.label.is_empty() {
            len += 1;
        }
        if !self.kind.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        if !self.members.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.BusNotModeled", len)?;
        if !self.label.is_empty() {
            struct_ser.serialize_field("label", &self.label)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        if !self.members.is_empty() {
            struct_ser.serialize_field("members", &self.members)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BusNotModeled {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "label",
            "kind",
            "prov",
            "members",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Label,
            Kind,
            Prov,
            Members,
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
                            "label" => Ok(GeneratedField::Label),
                            "kind" => Ok(GeneratedField::Kind),
                            "prov" => Ok(GeneratedField::Prov),
                            "members" => Ok(GeneratedField::Members),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BusNotModeled;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.BusNotModeled")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BusNotModeled, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut label__ = None;
                let mut kind__ = None;
                let mut prov__ = None;
                let mut members__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Label => {
                            if label__.is_some() {
                                return Err(serde::de::Error::duplicate_field("label"));
                            }
                            label__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                        GeneratedField::Members => {
                            if members__.is_some() {
                                return Err(serde::de::Error::duplicate_field("members"));
                            }
                            members__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(BusNotModeled {
                    label: label__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    prov: prov__,
                    members: members__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.BusNotModeled", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ClassSource {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "CLASS_SOURCE_UNSPECIFIED",
            Self::Convention => "CLASS_SOURCE_CONVENTION",
            Self::Datasheet => "CLASS_SOURCE_DATASHEET",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for ClassSource {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "CLASS_SOURCE_UNSPECIFIED",
            "CLASS_SOURCE_CONVENTION",
            "CLASS_SOURCE_DATASHEET",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ClassSource;

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
                    "CLASS_SOURCE_UNSPECIFIED" => Ok(ClassSource::Unspecified),
                    "CLASS_SOURCE_CONVENTION" => Ok(ClassSource::Convention),
                    "CLASS_SOURCE_DATASHEET" => Ok(ClassSource::Datasheet),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for Component {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.ref_des.is_empty() {
            len += 1;
        }
        if !self.sections.is_empty() {
            len += 1;
        }
        if !self.footprint_ref.is_empty() {
            len += 1;
        }
        if !self.device_classes.is_empty() {
            len += 1;
        }
        if self.value.is_some() {
            len += 1;
        }
        if !self.mpn.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.Component", len)?;
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if !self.sections.is_empty() {
            struct_ser.serialize_field("sections", &self.sections)?;
        }
        if !self.footprint_ref.is_empty() {
            struct_ser.serialize_field("footprintRef", &self.footprint_ref)?;
        }
        if !self.device_classes.is_empty() {
            struct_ser.serialize_field("deviceClasses", &self.device_classes)?;
        }
        if let Some(v) = self.value.as_ref() {
            struct_ser.serialize_field("value", v)?;
        }
        if !self.mpn.is_empty() {
            struct_ser.serialize_field("mpn", &self.mpn)?;
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
impl<'de> serde::Deserialize<'de> for Component {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "ref_des",
            "refDes",
            "sections",
            "footprint_ref",
            "footprintRef",
            "device_classes",
            "deviceClasses",
            "value",
            "mpn",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            RefDes,
            Sections,
            FootprintRef,
            DeviceClasses,
            Value,
            Mpn,
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
                            "refDes" | "ref_des" => Ok(GeneratedField::RefDes),
                            "sections" => Ok(GeneratedField::Sections),
                            "footprintRef" | "footprint_ref" => Ok(GeneratedField::FootprintRef),
                            "deviceClasses" | "device_classes" => Ok(GeneratedField::DeviceClasses),
                            "value" => Ok(GeneratedField::Value),
                            "mpn" => Ok(GeneratedField::Mpn),
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
            type Value = Component;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.Component")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Component, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut ref_des__ = None;
                let mut sections__ = None;
                let mut footprint_ref__ = None;
                let mut device_classes__ = None;
                let mut value__ = None;
                let mut mpn__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Sections => {
                            if sections__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sections"));
                            }
                            sections__ = Some(map_.next_value()?);
                        }
                        GeneratedField::FootprintRef => {
                            if footprint_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("footprintRef"));
                            }
                            footprint_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::DeviceClasses => {
                            if device_classes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("deviceClasses"));
                            }
                            device_classes__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Value => {
                            if value__.is_some() {
                                return Err(serde::de::Error::duplicate_field("value"));
                            }
                            value__ = map_.next_value()?;
                        }
                        GeneratedField::Mpn => {
                            if mpn__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mpn"));
                            }
                            mpn__ = Some(map_.next_value()?);
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
                Ok(Component {
                    ref_des: ref_des__.unwrap_or_default(),
                    sections: sections__.unwrap_or_default(),
                    footprint_ref: footprint_ref__.unwrap_or_default(),
                    device_classes: device_classes__.unwrap_or_default(),
                    value: value__,
                    mpn: mpn__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.Component", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ComponentClassTag {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.class.is_empty() {
            len += 1;
        }
        if self.source != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.ComponentClassTag", len)?;
        if !self.class.is_empty() {
            struct_ser.serialize_field("class", &self.class)?;
        }
        if self.source != 0 {
            let v = ClassSource::try_from(self.source)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.source)))?;
            struct_ser.serialize_field("source", &v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ComponentClassTag {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "class",
            "source",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Class,
            Source,
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
                            "class" => Ok(GeneratedField::Class),
                            "source" => Ok(GeneratedField::Source),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ComponentClassTag;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.ComponentClassTag")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ComponentClassTag, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut class__ = None;
                let mut source__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Class => {
                            if class__.is_some() {
                                return Err(serde::de::Error::duplicate_field("class"));
                            }
                            class__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Source => {
                            if source__.is_some() {
                                return Err(serde::de::Error::duplicate_field("source"));
                            }
                            source__ = Some(map_.next_value::<ClassSource>()? as i32);
                        }
                    }
                }
                Ok(ComponentClassTag {
                    class: class__.unwrap_or_default(),
                    source: source__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.ComponentClassTag", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ComponentSection {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.index != 0 {
            len += 1;
        }
        if !self.part_ref.is_empty() {
            len += 1;
        }
        if !self.library_ref.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.ComponentSection", len)?;
        if self.index != 0 {
            struct_ser.serialize_field("index", &self.index)?;
        }
        if !self.part_ref.is_empty() {
            struct_ser.serialize_field("partRef", &self.part_ref)?;
        }
        if !self.library_ref.is_empty() {
            struct_ser.serialize_field("libraryRef", &self.library_ref)?;
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
impl<'de> serde::Deserialize<'de> for ComponentSection {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "index",
            "part_ref",
            "partRef",
            "library_ref",
            "libraryRef",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Index,
            PartRef,
            LibraryRef,
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
                            "index" => Ok(GeneratedField::Index),
                            "partRef" | "part_ref" => Ok(GeneratedField::PartRef),
                            "libraryRef" | "library_ref" => Ok(GeneratedField::LibraryRef),
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
            type Value = ComponentSection;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.ComponentSection")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ComponentSection, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut index__ = None;
                let mut part_ref__ = None;
                let mut library_ref__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Index => {
                            if index__.is_some() {
                                return Err(serde::de::Error::duplicate_field("index"));
                            }
                            index__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::PartRef => {
                            if part_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("partRef"));
                            }
                            part_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::LibraryRef => {
                            if library_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("libraryRef"));
                            }
                            library_ref__ = Some(map_.next_value()?);
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
                Ok(ComponentSection {
                    index: index__.unwrap_or_default(),
                    part_ref: part_ref__.unwrap_or_default(),
                    library_ref: library_ref__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.ComponentSection", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Connection {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.component_ref.is_empty() {
            len += 1;
        }
        if !self.pin_ref.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.Connection", len)?;
        if !self.component_ref.is_empty() {
            struct_ser.serialize_field("componentRef", &self.component_ref)?;
        }
        if !self.pin_ref.is_empty() {
            struct_ser.serialize_field("pinRef", &self.pin_ref)?;
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
impl<'de> serde::Deserialize<'de> for Connection {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "component_ref",
            "componentRef",
            "pin_ref",
            "pinRef",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            ComponentRef,
            PinRef,
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
                            "componentRef" | "component_ref" => Ok(GeneratedField::ComponentRef),
                            "pinRef" | "pin_ref" => Ok(GeneratedField::PinRef),
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
            type Value = Connection;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.Connection")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Connection, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut component_ref__ = None;
                let mut pin_ref__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::ComponentRef => {
                            if component_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("componentRef"));
                            }
                            component_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::PinRef => {
                            if pin_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pinRef"));
                            }
                            pin_ref__ = Some(map_.next_value()?);
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
                Ok(Connection {
                    component_ref: component_ref__.unwrap_or_default(),
                    pin_ref: pin_ref__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.Connection", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Constraint {
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
        if !self.kind.is_empty() {
            len += 1;
        }
        if !self.params.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.Constraint", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if !self.params.is_empty() {
            struct_ser.serialize_field("params", &self.params)?;
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
impl<'de> serde::Deserialize<'de> for Constraint {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "kind",
            "params",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Kind,
            Params,
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
                            "name" => Ok(GeneratedField::Name),
                            "kind" => Ok(GeneratedField::Kind),
                            "params" => Ok(GeneratedField::Params),
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
            type Value = Constraint;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.Constraint")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Constraint, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut kind__ = None;
                let mut params__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Params => {
                            if params__.is_some() {
                                return Err(serde::de::Error::duplicate_field("params"));
                            }
                            params__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
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
                Ok(Constraint {
                    name: name__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    params: params__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.Constraint", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DanglingEndpoint {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.x != 0 {
            len += 1;
        }
        if self.y != 0 {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.DanglingEndpoint", len)?;
        if self.x != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("x", ToString::to_string(&self.x).as_str())?;
        }
        if self.y != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("y", ToString::to_string(&self.y).as_str())?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DanglingEndpoint {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "x",
            "y",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            X,
            Y,
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
                            "x" => Ok(GeneratedField::X),
                            "y" => Ok(GeneratedField::Y),
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
            type Value = DanglingEndpoint;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.DanglingEndpoint")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DanglingEndpoint, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut x__ = None;
                let mut y__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::X => {
                            if x__.is_some() {
                                return Err(serde::de::Error::duplicate_field("x"));
                            }
                            x__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Y => {
                            if y__.is_some() {
                                return Err(serde::de::Error::duplicate_field("y"));
                            }
                            y__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(DanglingEndpoint {
                    x: x__.unwrap_or_default(),
                    y: y__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.DanglingEndpoint", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Design {
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
        if !self.ir_version.is_empty() {
            len += 1;
        }
        if !self.source_format.is_empty() {
            len += 1;
        }
        if !self.libraries.is_empty() {
            len += 1;
        }
        if !self.components.is_empty() {
            len += 1;
        }
        if !self.nets.is_empty() {
            len += 1;
        }
        if !self.sheets.is_empty() {
            len += 1;
        }
        if self.input_diagnostics.is_some() {
            len += 1;
        }
        if !self.footprints.is_empty() {
            len += 1;
        }
        if !self.layers.is_empty() {
            len += 1;
        }
        if !self.constraints.is_empty() {
            len += 1;
        }
        if !self.bom.is_empty() {
            len += 1;
        }
        if !self.net_groups.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if !self.fidelity.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.Design", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.ir_version.is_empty() {
            struct_ser.serialize_field("irVersion", &self.ir_version)?;
        }
        if !self.source_format.is_empty() {
            struct_ser.serialize_field("sourceFormat", &self.source_format)?;
        }
        if !self.libraries.is_empty() {
            struct_ser.serialize_field("libraries", &self.libraries)?;
        }
        if !self.components.is_empty() {
            struct_ser.serialize_field("components", &self.components)?;
        }
        if !self.nets.is_empty() {
            struct_ser.serialize_field("nets", &self.nets)?;
        }
        if !self.sheets.is_empty() {
            struct_ser.serialize_field("sheets", &self.sheets)?;
        }
        if let Some(v) = self.input_diagnostics.as_ref() {
            struct_ser.serialize_field("inputDiagnostics", v)?;
        }
        if !self.footprints.is_empty() {
            struct_ser.serialize_field("footprints", &self.footprints)?;
        }
        if !self.layers.is_empty() {
            struct_ser.serialize_field("layers", &self.layers)?;
        }
        if !self.constraints.is_empty() {
            struct_ser.serialize_field("constraints", &self.constraints)?;
        }
        if !self.bom.is_empty() {
            struct_ser.serialize_field("bom", &self.bom)?;
        }
        if !self.net_groups.is_empty() {
            struct_ser.serialize_field("netGroups", &self.net_groups)?;
        }
        if !self.attributes.is_empty() {
            struct_ser.serialize_field("attributes", &self.attributes)?;
        }
        if !self.fidelity.is_empty() {
            struct_ser.serialize_field("fidelity", &self.fidelity)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Design {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "ir_version",
            "irVersion",
            "source_format",
            "sourceFormat",
            "libraries",
            "components",
            "nets",
            "sheets",
            "input_diagnostics",
            "inputDiagnostics",
            "footprints",
            "layers",
            "constraints",
            "bom",
            "net_groups",
            "netGroups",
            "attributes",
            "fidelity",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            IrVersion,
            SourceFormat,
            Libraries,
            Components,
            Nets,
            Sheets,
            InputDiagnostics,
            Footprints,
            Layers,
            Constraints,
            Bom,
            NetGroups,
            Attributes,
            Fidelity,
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
                            "name" => Ok(GeneratedField::Name),
                            "irVersion" | "ir_version" => Ok(GeneratedField::IrVersion),
                            "sourceFormat" | "source_format" => Ok(GeneratedField::SourceFormat),
                            "libraries" => Ok(GeneratedField::Libraries),
                            "components" => Ok(GeneratedField::Components),
                            "nets" => Ok(GeneratedField::Nets),
                            "sheets" => Ok(GeneratedField::Sheets),
                            "inputDiagnostics" | "input_diagnostics" => Ok(GeneratedField::InputDiagnostics),
                            "footprints" => Ok(GeneratedField::Footprints),
                            "layers" => Ok(GeneratedField::Layers),
                            "constraints" => Ok(GeneratedField::Constraints),
                            "bom" => Ok(GeneratedField::Bom),
                            "netGroups" | "net_groups" => Ok(GeneratedField::NetGroups),
                            "attributes" => Ok(GeneratedField::Attributes),
                            "fidelity" => Ok(GeneratedField::Fidelity),
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
            type Value = Design;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.Design")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Design, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut ir_version__ = None;
                let mut source_format__ = None;
                let mut libraries__ = None;
                let mut components__ = None;
                let mut nets__ = None;
                let mut sheets__ = None;
                let mut input_diagnostics__ = None;
                let mut footprints__ = None;
                let mut layers__ = None;
                let mut constraints__ = None;
                let mut bom__ = None;
                let mut net_groups__ = None;
                let mut attributes__ = None;
                let mut fidelity__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::IrVersion => {
                            if ir_version__.is_some() {
                                return Err(serde::de::Error::duplicate_field("irVersion"));
                            }
                            ir_version__ = Some(map_.next_value()?);
                        }
                        GeneratedField::SourceFormat => {
                            if source_format__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sourceFormat"));
                            }
                            source_format__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Libraries => {
                            if libraries__.is_some() {
                                return Err(serde::de::Error::duplicate_field("libraries"));
                            }
                            libraries__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Components => {
                            if components__.is_some() {
                                return Err(serde::de::Error::duplicate_field("components"));
                            }
                            components__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Nets => {
                            if nets__.is_some() {
                                return Err(serde::de::Error::duplicate_field("nets"));
                            }
                            nets__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Sheets => {
                            if sheets__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sheets"));
                            }
                            sheets__ = Some(map_.next_value()?);
                        }
                        GeneratedField::InputDiagnostics => {
                            if input_diagnostics__.is_some() {
                                return Err(serde::de::Error::duplicate_field("inputDiagnostics"));
                            }
                            input_diagnostics__ = map_.next_value()?;
                        }
                        GeneratedField::Footprints => {
                            if footprints__.is_some() {
                                return Err(serde::de::Error::duplicate_field("footprints"));
                            }
                            footprints__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Layers => {
                            if layers__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layers"));
                            }
                            layers__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Constraints => {
                            if constraints__.is_some() {
                                return Err(serde::de::Error::duplicate_field("constraints"));
                            }
                            constraints__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Bom => {
                            if bom__.is_some() {
                                return Err(serde::de::Error::duplicate_field("bom"));
                            }
                            bom__ = Some(map_.next_value()?);
                        }
                        GeneratedField::NetGroups => {
                            if net_groups__.is_some() {
                                return Err(serde::de::Error::duplicate_field("netGroups"));
                            }
                            net_groups__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Attributes => {
                            if attributes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("attributes"));
                            }
                            attributes__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                        GeneratedField::Fidelity => {
                            if fidelity__.is_some() {
                                return Err(serde::de::Error::duplicate_field("fidelity"));
                            }
                            fidelity__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(Design {
                    name: name__.unwrap_or_default(),
                    ir_version: ir_version__.unwrap_or_default(),
                    source_format: source_format__.unwrap_or_default(),
                    libraries: libraries__.unwrap_or_default(),
                    components: components__.unwrap_or_default(),
                    nets: nets__.unwrap_or_default(),
                    sheets: sheets__.unwrap_or_default(),
                    input_diagnostics: input_diagnostics__,
                    footprints: footprints__.unwrap_or_default(),
                    layers: layers__.unwrap_or_default(),
                    constraints: constraints__.unwrap_or_default(),
                    bom: bom__.unwrap_or_default(),
                    net_groups: net_groups__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    fidelity: fidelity__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.Design", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for FidelityFragment {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.prov.is_some() {
            len += 1;
        }
        if !self.format.is_empty() {
            len += 1;
        }
        if !self.raw.is_empty() {
            len += 1;
        }
        if !self.note.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.FidelityFragment", len)?;
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        if !self.format.is_empty() {
            struct_ser.serialize_field("format", &self.format)?;
        }
        if !self.raw.is_empty() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("raw", pbjson::private::base64::encode(&self.raw).as_str())?;
        }
        if !self.note.is_empty() {
            struct_ser.serialize_field("note", &self.note)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for FidelityFragment {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "prov",
            "format",
            "raw",
            "note",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Prov,
            Format,
            Raw,
            Note,
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
                            "prov" => Ok(GeneratedField::Prov),
                            "format" => Ok(GeneratedField::Format),
                            "raw" => Ok(GeneratedField::Raw),
                            "note" => Ok(GeneratedField::Note),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = FidelityFragment;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.FidelityFragment")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<FidelityFragment, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut prov__ = None;
                let mut format__ = None;
                let mut raw__ = None;
                let mut note__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                        GeneratedField::Format => {
                            if format__.is_some() {
                                return Err(serde::de::Error::duplicate_field("format"));
                            }
                            format__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Raw => {
                            if raw__.is_some() {
                                return Err(serde::de::Error::duplicate_field("raw"));
                            }
                            raw__ = 
                                Some(map_.next_value::<::pbjson::private::BytesDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Note => {
                            if note__.is_some() {
                                return Err(serde::de::Error::duplicate_field("note"));
                            }
                            note__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(FidelityFragment {
                    prov: prov__,
                    format: format__.unwrap_or_default(),
                    raw: raw__.unwrap_or_default(),
                    note: note__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.FidelityFragment", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Footprint {
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
        if !self.library.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.Footprint", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.library.is_empty() {
            struct_ser.serialize_field("library", &self.library)?;
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
impl<'de> serde::Deserialize<'de> for Footprint {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "library",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Library,
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
                            "name" => Ok(GeneratedField::Name),
                            "library" => Ok(GeneratedField::Library),
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
            type Value = Footprint;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.Footprint")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Footprint, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut library__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Library => {
                            if library__.is_some() {
                                return Err(serde::de::Error::duplicate_field("library"));
                            }
                            library__ = Some(map_.next_value()?);
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
                Ok(Footprint {
                    name: name__.unwrap_or_default(),
                    library: library__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.Footprint", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for GroupBus {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.label.is_empty() {
            len += 1;
        }
        if !self.members.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.GroupBus", len)?;
        if !self.label.is_empty() {
            struct_ser.serialize_field("label", &self.label)?;
        }
        if !self.members.is_empty() {
            struct_ser.serialize_field("members", &self.members)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for GroupBus {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "label",
            "members",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Label,
            Members,
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
                            "label" => Ok(GeneratedField::Label),
                            "members" => Ok(GeneratedField::Members),
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
            type Value = GroupBus;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.GroupBus")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<GroupBus, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut label__ = None;
                let mut members__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Label => {
                            if label__.is_some() {
                                return Err(serde::de::Error::duplicate_field("label"));
                            }
                            label__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Members => {
                            if members__.is_some() {
                                return Err(serde::de::Error::duplicate_field("members"));
                            }
                            members__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(GroupBus {
                    label: label__.unwrap_or_default(),
                    members: members__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.GroupBus", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for InputDiagnostics {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.dangling_endpoints.is_empty() {
            len += 1;
        }
        if !self.ref_des_collisions.is_empty() {
            len += 1;
        }
        if !self.no_junction_endpoints.is_empty() {
            len += 1;
        }
        if !self.unmodeled_buses.is_empty() {
            len += 1;
        }
        if !self.unresolved_symbols.is_empty() {
            len += 1;
        }
        if !self.resolved_symbols.is_empty() {
            len += 1;
        }
        if !self.joined_taps.is_empty() {
            len += 1;
        }
        if !self.unannotated_components.is_empty() {
            len += 1;
        }
        if !self.unexpanded_hierarchy.is_empty() {
            len += 1;
        }
        if !self.range_labels.is_empty() {
            len += 1;
        }
        if !self.group_buses.is_empty() {
            len += 1;
        }
        if !self.supplied.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.InputDiagnostics", len)?;
        if !self.dangling_endpoints.is_empty() {
            struct_ser.serialize_field("danglingEndpoints", &self.dangling_endpoints)?;
        }
        if !self.ref_des_collisions.is_empty() {
            struct_ser.serialize_field("refDesCollisions", &self.ref_des_collisions)?;
        }
        if !self.no_junction_endpoints.is_empty() {
            struct_ser.serialize_field("noJunctionEndpoints", &self.no_junction_endpoints)?;
        }
        if !self.unmodeled_buses.is_empty() {
            struct_ser.serialize_field("unmodeledBuses", &self.unmodeled_buses)?;
        }
        if !self.unresolved_symbols.is_empty() {
            struct_ser.serialize_field("unresolvedSymbols", &self.unresolved_symbols)?;
        }
        if !self.resolved_symbols.is_empty() {
            struct_ser.serialize_field("resolvedSymbols", &self.resolved_symbols)?;
        }
        if !self.joined_taps.is_empty() {
            struct_ser.serialize_field("joinedTaps", &self.joined_taps)?;
        }
        if !self.unannotated_components.is_empty() {
            struct_ser.serialize_field("unannotatedComponents", &self.unannotated_components)?;
        }
        if !self.unexpanded_hierarchy.is_empty() {
            struct_ser.serialize_field("unexpandedHierarchy", &self.unexpanded_hierarchy)?;
        }
        if !self.range_labels.is_empty() {
            struct_ser.serialize_field("rangeLabels", &self.range_labels)?;
        }
        if !self.group_buses.is_empty() {
            struct_ser.serialize_field("groupBuses", &self.group_buses)?;
        }
        if !self.supplied.is_empty() {
            struct_ser.serialize_field("supplied", &self.supplied)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for InputDiagnostics {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "dangling_endpoints",
            "danglingEndpoints",
            "ref_des_collisions",
            "refDesCollisions",
            "no_junction_endpoints",
            "noJunctionEndpoints",
            "unmodeled_buses",
            "unmodeledBuses",
            "unresolved_symbols",
            "unresolvedSymbols",
            "resolved_symbols",
            "resolvedSymbols",
            "joined_taps",
            "joinedTaps",
            "unannotated_components",
            "unannotatedComponents",
            "unexpanded_hierarchy",
            "unexpandedHierarchy",
            "range_labels",
            "rangeLabels",
            "group_buses",
            "groupBuses",
            "supplied",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            DanglingEndpoints,
            RefDesCollisions,
            NoJunctionEndpoints,
            UnmodeledBuses,
            UnresolvedSymbols,
            ResolvedSymbols,
            JoinedTaps,
            UnannotatedComponents,
            UnexpandedHierarchy,
            RangeLabels,
            GroupBuses,
            Supplied,
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
                            "danglingEndpoints" | "dangling_endpoints" => Ok(GeneratedField::DanglingEndpoints),
                            "refDesCollisions" | "ref_des_collisions" => Ok(GeneratedField::RefDesCollisions),
                            "noJunctionEndpoints" | "no_junction_endpoints" => Ok(GeneratedField::NoJunctionEndpoints),
                            "unmodeledBuses" | "unmodeled_buses" => Ok(GeneratedField::UnmodeledBuses),
                            "unresolvedSymbols" | "unresolved_symbols" => Ok(GeneratedField::UnresolvedSymbols),
                            "resolvedSymbols" | "resolved_symbols" => Ok(GeneratedField::ResolvedSymbols),
                            "joinedTaps" | "joined_taps" => Ok(GeneratedField::JoinedTaps),
                            "unannotatedComponents" | "unannotated_components" => Ok(GeneratedField::UnannotatedComponents),
                            "unexpandedHierarchy" | "unexpanded_hierarchy" => Ok(GeneratedField::UnexpandedHierarchy),
                            "rangeLabels" | "range_labels" => Ok(GeneratedField::RangeLabels),
                            "groupBuses" | "group_buses" => Ok(GeneratedField::GroupBuses),
                            "supplied" => Ok(GeneratedField::Supplied),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = InputDiagnostics;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.InputDiagnostics")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<InputDiagnostics, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut dangling_endpoints__ = None;
                let mut ref_des_collisions__ = None;
                let mut no_junction_endpoints__ = None;
                let mut unmodeled_buses__ = None;
                let mut unresolved_symbols__ = None;
                let mut resolved_symbols__ = None;
                let mut joined_taps__ = None;
                let mut unannotated_components__ = None;
                let mut unexpanded_hierarchy__ = None;
                let mut range_labels__ = None;
                let mut group_buses__ = None;
                let mut supplied__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::DanglingEndpoints => {
                            if dangling_endpoints__.is_some() {
                                return Err(serde::de::Error::duplicate_field("danglingEndpoints"));
                            }
                            dangling_endpoints__ = Some(map_.next_value()?);
                        }
                        GeneratedField::RefDesCollisions => {
                            if ref_des_collisions__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDesCollisions"));
                            }
                            ref_des_collisions__ = Some(map_.next_value()?);
                        }
                        GeneratedField::NoJunctionEndpoints => {
                            if no_junction_endpoints__.is_some() {
                                return Err(serde::de::Error::duplicate_field("noJunctionEndpoints"));
                            }
                            no_junction_endpoints__ = Some(map_.next_value()?);
                        }
                        GeneratedField::UnmodeledBuses => {
                            if unmodeled_buses__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unmodeledBuses"));
                            }
                            unmodeled_buses__ = Some(map_.next_value()?);
                        }
                        GeneratedField::UnresolvedSymbols => {
                            if unresolved_symbols__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unresolvedSymbols"));
                            }
                            unresolved_symbols__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ResolvedSymbols => {
                            if resolved_symbols__.is_some() {
                                return Err(serde::de::Error::duplicate_field("resolvedSymbols"));
                            }
                            resolved_symbols__ = Some(map_.next_value()?);
                        }
                        GeneratedField::JoinedTaps => {
                            if joined_taps__.is_some() {
                                return Err(serde::de::Error::duplicate_field("joinedTaps"));
                            }
                            joined_taps__ = Some(map_.next_value()?);
                        }
                        GeneratedField::UnannotatedComponents => {
                            if unannotated_components__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unannotatedComponents"));
                            }
                            unannotated_components__ = Some(map_.next_value()?);
                        }
                        GeneratedField::UnexpandedHierarchy => {
                            if unexpanded_hierarchy__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unexpandedHierarchy"));
                            }
                            unexpanded_hierarchy__ = Some(map_.next_value()?);
                        }
                        GeneratedField::RangeLabels => {
                            if range_labels__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rangeLabels"));
                            }
                            range_labels__ = Some(map_.next_value()?);
                        }
                        GeneratedField::GroupBuses => {
                            if group_buses__.is_some() {
                                return Err(serde::de::Error::duplicate_field("groupBuses"));
                            }
                            group_buses__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Supplied => {
                            if supplied__.is_some() {
                                return Err(serde::de::Error::duplicate_field("supplied"));
                            }
                            supplied__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(InputDiagnostics {
                    dangling_endpoints: dangling_endpoints__.unwrap_or_default(),
                    ref_des_collisions: ref_des_collisions__.unwrap_or_default(),
                    no_junction_endpoints: no_junction_endpoints__.unwrap_or_default(),
                    unmodeled_buses: unmodeled_buses__.unwrap_or_default(),
                    unresolved_symbols: unresolved_symbols__.unwrap_or_default(),
                    resolved_symbols: resolved_symbols__.unwrap_or_default(),
                    joined_taps: joined_taps__.unwrap_or_default(),
                    unannotated_components: unannotated_components__.unwrap_or_default(),
                    unexpanded_hierarchy: unexpanded_hierarchy__.unwrap_or_default(),
                    range_labels: range_labels__.unwrap_or_default(),
                    group_buses: group_buses__.unwrap_or_default(),
                    supplied: supplied__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.InputDiagnostics", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for JoinedTap {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.x != 0 {
            len += 1;
        }
        if self.y != 0 {
            len += 1;
        }
        if !self.join_kind.is_empty() {
            len += 1;
        }
        if !self.label.is_empty() {
            len += 1;
        }
        if self.segments != 0 {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.JoinedTap", len)?;
        if self.x != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("x", ToString::to_string(&self.x).as_str())?;
        }
        if self.y != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("y", ToString::to_string(&self.y).as_str())?;
        }
        if !self.join_kind.is_empty() {
            struct_ser.serialize_field("joinKind", &self.join_kind)?;
        }
        if !self.label.is_empty() {
            struct_ser.serialize_field("label", &self.label)?;
        }
        if self.segments != 0 {
            struct_ser.serialize_field("segments", &self.segments)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for JoinedTap {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "x",
            "y",
            "join_kind",
            "joinKind",
            "label",
            "segments",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            X,
            Y,
            JoinKind,
            Label,
            Segments,
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
                            "x" => Ok(GeneratedField::X),
                            "y" => Ok(GeneratedField::Y),
                            "joinKind" | "join_kind" => Ok(GeneratedField::JoinKind),
                            "label" => Ok(GeneratedField::Label),
                            "segments" => Ok(GeneratedField::Segments),
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
            type Value = JoinedTap;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.JoinedTap")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<JoinedTap, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut x__ = None;
                let mut y__ = None;
                let mut join_kind__ = None;
                let mut label__ = None;
                let mut segments__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::X => {
                            if x__.is_some() {
                                return Err(serde::de::Error::duplicate_field("x"));
                            }
                            x__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Y => {
                            if y__.is_some() {
                                return Err(serde::de::Error::duplicate_field("y"));
                            }
                            y__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::JoinKind => {
                            if join_kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("joinKind"));
                            }
                            join_kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Label => {
                            if label__.is_some() {
                                return Err(serde::de::Error::duplicate_field("label"));
                            }
                            label__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Segments => {
                            if segments__.is_some() {
                                return Err(serde::de::Error::duplicate_field("segments"));
                            }
                            segments__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(JoinedTap {
                    x: x__.unwrap_or_default(),
                    y: y__.unwrap_or_default(),
                    join_kind: join_kind__.unwrap_or_default(),
                    label: label__.unwrap_or_default(),
                    segments: segments__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.JoinedTap", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Layer {
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
        if self.index != 0 {
            len += 1;
        }
        if self.function != 0 {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.Layer", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if self.index != 0 {
            struct_ser.serialize_field("index", &self.index)?;
        }
        if self.function != 0 {
            let v = LayerFunction::try_from(self.function)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.function)))?;
            struct_ser.serialize_field("function", &v)?;
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
impl<'de> serde::Deserialize<'de> for Layer {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "index",
            "function",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Index,
            Function,
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
                            "name" => Ok(GeneratedField::Name),
                            "index" => Ok(GeneratedField::Index),
                            "function" => Ok(GeneratedField::Function),
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
            type Value = Layer;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.Layer")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Layer, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut index__ = None;
                let mut function__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Index => {
                            if index__.is_some() {
                                return Err(serde::de::Error::duplicate_field("index"));
                            }
                            index__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Function => {
                            if function__.is_some() {
                                return Err(serde::de::Error::duplicate_field("function"));
                            }
                            function__ = Some(map_.next_value::<LayerFunction>()? as i32);
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
                Ok(Layer {
                    name: name__.unwrap_or_default(),
                    index: index__.unwrap_or_default(),
                    function: function__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.Layer", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for LayerFunction {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "LAYER_FUNCTION_UNSPECIFIED",
            Self::Signal => "LAYER_FUNCTION_SIGNAL",
            Self::Plane => "LAYER_FUNCTION_PLANE",
            Self::Dielectric => "LAYER_FUNCTION_DIELECTRIC",
            Self::SolderMask => "LAYER_FUNCTION_SOLDER_MASK",
            Self::Silkscreen => "LAYER_FUNCTION_SILKSCREEN",
            Self::Paste => "LAYER_FUNCTION_PASTE",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for LayerFunction {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "LAYER_FUNCTION_UNSPECIFIED",
            "LAYER_FUNCTION_SIGNAL",
            "LAYER_FUNCTION_PLANE",
            "LAYER_FUNCTION_DIELECTRIC",
            "LAYER_FUNCTION_SOLDER_MASK",
            "LAYER_FUNCTION_SILKSCREEN",
            "LAYER_FUNCTION_PASTE",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = LayerFunction;

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
                    "LAYER_FUNCTION_UNSPECIFIED" => Ok(LayerFunction::Unspecified),
                    "LAYER_FUNCTION_SIGNAL" => Ok(LayerFunction::Signal),
                    "LAYER_FUNCTION_PLANE" => Ok(LayerFunction::Plane),
                    "LAYER_FUNCTION_DIELECTRIC" => Ok(LayerFunction::Dielectric),
                    "LAYER_FUNCTION_SOLDER_MASK" => Ok(LayerFunction::SolderMask),
                    "LAYER_FUNCTION_SILKSCREEN" => Ok(LayerFunction::Silkscreen),
                    "LAYER_FUNCTION_PASTE" => Ok(LayerFunction::Paste),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for Net {
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
        if !self.net_classes.is_empty() {
            len += 1;
        }
        if !self.connections.is_empty() {
            len += 1;
        }
        if !self.id.is_empty() {
            len += 1;
        }
        if !self.roles.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.Net", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.net_classes.is_empty() {
            struct_ser.serialize_field("netClasses", &self.net_classes)?;
        }
        if !self.connections.is_empty() {
            struct_ser.serialize_field("connections", &self.connections)?;
        }
        if !self.id.is_empty() {
            struct_ser.serialize_field("id", &self.id)?;
        }
        if !self.roles.is_empty() {
            struct_ser.serialize_field("roles", &self.roles)?;
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
impl<'de> serde::Deserialize<'de> for Net {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "net_classes",
            "netClasses",
            "connections",
            "id",
            "roles",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            NetClasses,
            Connections,
            Id,
            Roles,
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
                            "name" => Ok(GeneratedField::Name),
                            "netClasses" | "net_classes" => Ok(GeneratedField::NetClasses),
                            "connections" => Ok(GeneratedField::Connections),
                            "id" => Ok(GeneratedField::Id),
                            "roles" => Ok(GeneratedField::Roles),
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
            type Value = Net;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.Net")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Net, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut net_classes__ = None;
                let mut connections__ = None;
                let mut id__ = None;
                let mut roles__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::NetClasses => {
                            if net_classes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("netClasses"));
                            }
                            net_classes__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Connections => {
                            if connections__.is_some() {
                                return Err(serde::de::Error::duplicate_field("connections"));
                            }
                            connections__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Id => {
                            if id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("id"));
                            }
                            id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Roles => {
                            if roles__.is_some() {
                                return Err(serde::de::Error::duplicate_field("roles"));
                            }
                            roles__ = Some(map_.next_value()?);
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
                Ok(Net {
                    name: name__.unwrap_or_default(),
                    net_classes: net_classes__.unwrap_or_default(),
                    connections: connections__.unwrap_or_default(),
                    id: id__.unwrap_or_default(),
                    roles: roles__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.Net", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for NetGroup {
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
        if self.kind != 0 {
            len += 1;
        }
        if !self.nets.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.NetGroup", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if self.kind != 0 {
            let v = NetGroupKind::try_from(self.kind)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.kind)))?;
            struct_ser.serialize_field("kind", &v)?;
        }
        if !self.nets.is_empty() {
            struct_ser.serialize_field("nets", &self.nets)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for NetGroup {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "kind",
            "nets",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Kind,
            Nets,
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
                            "name" => Ok(GeneratedField::Name),
                            "kind" => Ok(GeneratedField::Kind),
                            "nets" => Ok(GeneratedField::Nets),
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
            type Value = NetGroup;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.NetGroup")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<NetGroup, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut kind__ = None;
                let mut nets__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value::<NetGroupKind>()? as i32);
                        }
                        GeneratedField::Nets => {
                            if nets__.is_some() {
                                return Err(serde::de::Error::duplicate_field("nets"));
                            }
                            nets__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(NetGroup {
                    name: name__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    nets: nets__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.NetGroup", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for NetGroupKind {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "NET_GROUP_KIND_UNSPECIFIED",
            Self::DiffPair => "NET_GROUP_KIND_DIFF_PAIR",
            Self::MatchGroup => "NET_GROUP_KIND_MATCH_GROUP",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for NetGroupKind {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "NET_GROUP_KIND_UNSPECIFIED",
            "NET_GROUP_KIND_DIFF_PAIR",
            "NET_GROUP_KIND_MATCH_GROUP",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = NetGroupKind;

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
                    "NET_GROUP_KIND_UNSPECIFIED" => Ok(NetGroupKind::Unspecified),
                    "NET_GROUP_KIND_DIFF_PAIR" => Ok(NetGroupKind::DiffPair),
                    "NET_GROUP_KIND_MATCH_GROUP" => Ok(NetGroupKind::MatchGroup),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for NetRole {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.role_kind != 0 {
            len += 1;
        }
        if self.source != 0 {
            len += 1;
        }
        if !self.reason.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.NetRole", len)?;
        if self.role_kind != 0 {
            let v = Role::try_from(self.role_kind)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.role_kind)))?;
            struct_ser.serialize_field("roleKind", &v)?;
        }
        if self.source != 0 {
            let v = RoleSource::try_from(self.source)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.source)))?;
            struct_ser.serialize_field("source", &v)?;
        }
        if !self.reason.is_empty() {
            struct_ser.serialize_field("reason", &self.reason)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for NetRole {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "role_kind",
            "roleKind",
            "source",
            "reason",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            RoleKind,
            Source,
            Reason,
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
                            "roleKind" | "role_kind" => Ok(GeneratedField::RoleKind),
                            "source" => Ok(GeneratedField::Source),
                            "reason" => Ok(GeneratedField::Reason),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = NetRole;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.NetRole")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<NetRole, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut role_kind__ = None;
                let mut source__ = None;
                let mut reason__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::RoleKind => {
                            if role_kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("roleKind"));
                            }
                            role_kind__ = Some(map_.next_value::<Role>()? as i32);
                        }
                        GeneratedField::Source => {
                            if source__.is_some() {
                                return Err(serde::de::Error::duplicate_field("source"));
                            }
                            source__ = Some(map_.next_value::<RoleSource>()? as i32);
                        }
                        GeneratedField::Reason => {
                            if reason__.is_some() {
                                return Err(serde::de::Error::duplicate_field("reason"));
                            }
                            reason__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(NetRole {
                    role_kind: role_kind__.unwrap_or_default(),
                    source: source__.unwrap_or_default(),
                    reason: reason__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.NetRole", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PartLibrary {
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
        if !self.parts.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.PartLibrary", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.parts.is_empty() {
            struct_ser.serialize_field("parts", &self.parts)?;
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
impl<'de> serde::Deserialize<'de> for PartLibrary {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "parts",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Parts,
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
                            "name" => Ok(GeneratedField::Name),
                            "parts" => Ok(GeneratedField::Parts),
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
            type Value = PartLibrary;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.PartLibrary")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PartLibrary, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut parts__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Parts => {
                            if parts__.is_some() {
                                return Err(serde::de::Error::duplicate_field("parts"));
                            }
                            parts__ = Some(map_.next_value()?);
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
                Ok(PartLibrary {
                    name: name__.unwrap_or_default(),
                    parts: parts__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.PartLibrary", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PartType {
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
        if !self.kind.is_empty() {
            len += 1;
        }
        if !self.designator_prefix.is_empty() {
            len += 1;
        }
        if !self.pins.is_empty() {
            len += 1;
        }
        if !self.mpn.is_empty() {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.PartType", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if !self.designator_prefix.is_empty() {
            struct_ser.serialize_field("designatorPrefix", &self.designator_prefix)?;
        }
        if !self.pins.is_empty() {
            struct_ser.serialize_field("pins", &self.pins)?;
        }
        if !self.mpn.is_empty() {
            struct_ser.serialize_field("mpn", &self.mpn)?;
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
impl<'de> serde::Deserialize<'de> for PartType {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "kind",
            "designator_prefix",
            "designatorPrefix",
            "pins",
            "mpn",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Kind,
            DesignatorPrefix,
            Pins,
            Mpn,
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
                            "name" => Ok(GeneratedField::Name),
                            "kind" => Ok(GeneratedField::Kind),
                            "designatorPrefix" | "designator_prefix" => Ok(GeneratedField::DesignatorPrefix),
                            "pins" => Ok(GeneratedField::Pins),
                            "mpn" => Ok(GeneratedField::Mpn),
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
            type Value = PartType;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.PartType")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PartType, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut kind__ = None;
                let mut designator_prefix__ = None;
                let mut pins__ = None;
                let mut mpn__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::DesignatorPrefix => {
                            if designator_prefix__.is_some() {
                                return Err(serde::de::Error::duplicate_field("designatorPrefix"));
                            }
                            designator_prefix__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Pins => {
                            if pins__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pins"));
                            }
                            pins__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Mpn => {
                            if mpn__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mpn"));
                            }
                            mpn__ = Some(map_.next_value()?);
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
                Ok(PartType {
                    name: name__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    designator_prefix: designator_prefix__.unwrap_or_default(),
                    pins: pins__.unwrap_or_default(),
                    mpn: mpn__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.PartType", FIELDS, GeneratedVisitor)
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
        if !self.name.is_empty() {
            len += 1;
        }
        if !self.designator.is_empty() {
            len += 1;
        }
        if self.direction != 0 {
            len += 1;
        }
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.Pin", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.designator.is_empty() {
            struct_ser.serialize_field("designator", &self.designator)?;
        }
        if self.direction != 0 {
            let v = PinDirection::try_from(self.direction)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.direction)))?;
            struct_ser.serialize_field("direction", &v)?;
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
            "name",
            "designator",
            "direction",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Designator,
            Direction,
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
                            "name" => Ok(GeneratedField::Name),
                            "designator" => Ok(GeneratedField::Designator),
                            "direction" => Ok(GeneratedField::Direction),
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
                formatter.write_str("struct agni.v1.ir.Pin")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Pin, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut designator__ = None;
                let mut direction__ = None;
                let mut attributes__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Designator => {
                            if designator__.is_some() {
                                return Err(serde::de::Error::duplicate_field("designator"));
                            }
                            designator__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Direction => {
                            if direction__.is_some() {
                                return Err(serde::de::Error::duplicate_field("direction"));
                            }
                            direction__ = Some(map_.next_value::<PinDirection>()? as i32);
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
                    name: name__.unwrap_or_default(),
                    designator: designator__.unwrap_or_default(),
                    direction: direction__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.Pin", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PinDirection {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "PIN_DIRECTION_UNSPECIFIED",
            Self::Input => "PIN_DIRECTION_INPUT",
            Self::Output => "PIN_DIRECTION_OUTPUT",
            Self::Inout => "PIN_DIRECTION_INOUT",
            Self::Passive => "PIN_DIRECTION_PASSIVE",
            Self::Power => "PIN_DIRECTION_POWER",
            Self::NoConnect => "PIN_DIRECTION_NO_CONNECT",
            Self::PowerIn => "PIN_DIRECTION_POWER_IN",
            Self::PowerOut => "PIN_DIRECTION_POWER_OUT",
            Self::Tristate => "PIN_DIRECTION_TRISTATE",
            Self::OpenCollector => "PIN_DIRECTION_OPEN_COLLECTOR",
            Self::OpenEmitter => "PIN_DIRECTION_OPEN_EMITTER",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for PinDirection {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "PIN_DIRECTION_UNSPECIFIED",
            "PIN_DIRECTION_INPUT",
            "PIN_DIRECTION_OUTPUT",
            "PIN_DIRECTION_INOUT",
            "PIN_DIRECTION_PASSIVE",
            "PIN_DIRECTION_POWER",
            "PIN_DIRECTION_NO_CONNECT",
            "PIN_DIRECTION_POWER_IN",
            "PIN_DIRECTION_POWER_OUT",
            "PIN_DIRECTION_TRISTATE",
            "PIN_DIRECTION_OPEN_COLLECTOR",
            "PIN_DIRECTION_OPEN_EMITTER",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PinDirection;

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
                    "PIN_DIRECTION_UNSPECIFIED" => Ok(PinDirection::Unspecified),
                    "PIN_DIRECTION_INPUT" => Ok(PinDirection::Input),
                    "PIN_DIRECTION_OUTPUT" => Ok(PinDirection::Output),
                    "PIN_DIRECTION_INOUT" => Ok(PinDirection::Inout),
                    "PIN_DIRECTION_PASSIVE" => Ok(PinDirection::Passive),
                    "PIN_DIRECTION_POWER" => Ok(PinDirection::Power),
                    "PIN_DIRECTION_NO_CONNECT" => Ok(PinDirection::NoConnect),
                    "PIN_DIRECTION_POWER_IN" => Ok(PinDirection::PowerIn),
                    "PIN_DIRECTION_POWER_OUT" => Ok(PinDirection::PowerOut),
                    "PIN_DIRECTION_TRISTATE" => Ok(PinDirection::Tristate),
                    "PIN_DIRECTION_OPEN_COLLECTOR" => Ok(PinDirection::OpenCollector),
                    "PIN_DIRECTION_OPEN_EMITTER" => Ok(PinDirection::OpenEmitter),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for Provenance {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.source_file.is_empty() {
            len += 1;
        }
        if self.span.is_some() {
            len += 1;
        }
        if !self.native_id.is_empty() {
            len += 1;
        }
        if !self.native_id_kind.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.Provenance", len)?;
        if !self.source_file.is_empty() {
            struct_ser.serialize_field("sourceFile", &self.source_file)?;
        }
        if let Some(v) = self.span.as_ref() {
            struct_ser.serialize_field("span", v)?;
        }
        if !self.native_id.is_empty() {
            struct_ser.serialize_field("nativeId", &self.native_id)?;
        }
        if !self.native_id_kind.is_empty() {
            struct_ser.serialize_field("nativeIdKind", &self.native_id_kind)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Provenance {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "source_file",
            "sourceFile",
            "span",
            "native_id",
            "nativeId",
            "native_id_kind",
            "nativeIdKind",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            SourceFile,
            Span,
            NativeId,
            NativeIdKind,
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
                            "sourceFile" | "source_file" => Ok(GeneratedField::SourceFile),
                            "span" => Ok(GeneratedField::Span),
                            "nativeId" | "native_id" => Ok(GeneratedField::NativeId),
                            "nativeIdKind" | "native_id_kind" => Ok(GeneratedField::NativeIdKind),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Provenance;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.Provenance")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Provenance, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut source_file__ = None;
                let mut span__ = None;
                let mut native_id__ = None;
                let mut native_id_kind__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::SourceFile => {
                            if source_file__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sourceFile"));
                            }
                            source_file__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Span => {
                            if span__.is_some() {
                                return Err(serde::de::Error::duplicate_field("span"));
                            }
                            span__ = map_.next_value()?;
                        }
                        GeneratedField::NativeId => {
                            if native_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("nativeId"));
                            }
                            native_id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::NativeIdKind => {
                            if native_id_kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("nativeIdKind"));
                            }
                            native_id_kind__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Provenance {
                    source_file: source_file__.unwrap_or_default(),
                    span: span__,
                    native_id: native_id__.unwrap_or_default(),
                    native_id_kind: native_id_kind__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.Provenance", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Quantity {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.input.is_empty() {
            len += 1;
        }
        if self.value.is_some() {
            len += 1;
        }
        if !self.unit.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.Quantity", len)?;
        if !self.input.is_empty() {
            struct_ser.serialize_field("input", &self.input)?;
        }
        if let Some(v) = self.value.as_ref() {
            struct_ser.serialize_field("value", v)?;
        }
        if !self.unit.is_empty() {
            struct_ser.serialize_field("unit", &self.unit)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Quantity {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "input",
            "value",
            "unit",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Input,
            Value,
            Unit,
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
                            "input" => Ok(GeneratedField::Input),
                            "value" => Ok(GeneratedField::Value),
                            "unit" => Ok(GeneratedField::Unit),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Quantity;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.Quantity")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Quantity, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut input__ = None;
                let mut value__ = None;
                let mut unit__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Input => {
                            if input__.is_some() {
                                return Err(serde::de::Error::duplicate_field("input"));
                            }
                            input__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Value => {
                            if value__.is_some() {
                                return Err(serde::de::Error::duplicate_field("value"));
                            }
                            value__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                        GeneratedField::Unit => {
                            if unit__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unit"));
                            }
                            unit__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Quantity {
                    input: input__.unwrap_or_default(),
                    value: value__,
                    unit: unit__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.Quantity", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for RangeLabel {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.label.is_empty() {
            len += 1;
        }
        if self.reads_as_bus {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.RangeLabel", len)?;
        if !self.label.is_empty() {
            struct_ser.serialize_field("label", &self.label)?;
        }
        if self.reads_as_bus {
            struct_ser.serialize_field("readsAsBus", &self.reads_as_bus)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for RangeLabel {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "label",
            "reads_as_bus",
            "readsAsBus",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Label,
            ReadsAsBus,
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
                            "label" => Ok(GeneratedField::Label),
                            "readsAsBus" | "reads_as_bus" => Ok(GeneratedField::ReadsAsBus),
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
            type Value = RangeLabel;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.RangeLabel")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<RangeLabel, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut label__ = None;
                let mut reads_as_bus__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Label => {
                            if label__.is_some() {
                                return Err(serde::de::Error::duplicate_field("label"));
                            }
                            label__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ReadsAsBus => {
                            if reads_as_bus__.is_some() {
                                return Err(serde::de::Error::duplicate_field("readsAsBus"));
                            }
                            reads_as_bus__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(RangeLabel {
                    label: label__.unwrap_or_default(),
                    reads_as_bus: reads_as_bus__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.RangeLabel", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for RefDesCollision {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.ref_des.is_empty() {
            len += 1;
        }
        if !self.instances.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.RefDesCollision", len)?;
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if !self.instances.is_empty() {
            struct_ser.serialize_field("instances", &self.instances)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for RefDesCollision {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "ref_des",
            "refDes",
            "instances",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            RefDes,
            Instances,
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
                            "refDes" | "ref_des" => Ok(GeneratedField::RefDes),
                            "instances" => Ok(GeneratedField::Instances),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = RefDesCollision;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.RefDesCollision")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<RefDesCollision, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut ref_des__ = None;
                let mut instances__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Instances => {
                            if instances__.is_some() {
                                return Err(serde::de::Error::duplicate_field("instances"));
                            }
                            instances__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(RefDesCollision {
                    ref_des: ref_des__.unwrap_or_default(),
                    instances: instances__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.RefDesCollision", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ResolvedSymbol {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.symref.is_empty() {
            len += 1;
        }
        if !self.kind.is_empty() {
            len += 1;
        }
        if self.pin_count != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.ResolvedSymbol", len)?;
        if !self.symref.is_empty() {
            struct_ser.serialize_field("symref", &self.symref)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if self.pin_count != 0 {
            struct_ser.serialize_field("pinCount", &self.pin_count)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ResolvedSymbol {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "symref",
            "kind",
            "pin_count",
            "pinCount",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Symref,
            Kind,
            PinCount,
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
                            "symref" => Ok(GeneratedField::Symref),
                            "kind" => Ok(GeneratedField::Kind),
                            "pinCount" | "pin_count" => Ok(GeneratedField::PinCount),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ResolvedSymbol;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.ResolvedSymbol")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ResolvedSymbol, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut symref__ = None;
                let mut kind__ = None;
                let mut pin_count__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Symref => {
                            if symref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("symref"));
                            }
                            symref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::PinCount => {
                            if pin_count__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pinCount"));
                            }
                            pin_count__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(ResolvedSymbol {
                    symref: symref__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    pin_count: pin_count__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.ResolvedSymbol", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Role {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "ROLE_UNSPECIFIED",
            Self::Rail => "ROLE_RAIL",
            Self::Ground => "ROLE_GROUND",
            Self::Feedback => "ROLE_FEEDBACK",
            Self::Switching => "ROLE_SWITCHING",
            Self::Control => "ROLE_CONTROL",
            Self::GateDrive => "ROLE_GATE_DRIVE",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for Role {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "ROLE_UNSPECIFIED",
            "ROLE_RAIL",
            "ROLE_GROUND",
            "ROLE_FEEDBACK",
            "ROLE_SWITCHING",
            "ROLE_CONTROL",
            "ROLE_GATE_DRIVE",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Role;

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
                    "ROLE_UNSPECIFIED" => Ok(Role::Unspecified),
                    "ROLE_RAIL" => Ok(Role::Rail),
                    "ROLE_GROUND" => Ok(Role::Ground),
                    "ROLE_FEEDBACK" => Ok(Role::Feedback),
                    "ROLE_SWITCHING" => Ok(Role::Switching),
                    "ROLE_CONTROL" => Ok(Role::Control),
                    "ROLE_GATE_DRIVE" => Ok(Role::GateDrive),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for RoleSource {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "ROLE_SOURCE_UNSPECIFIED",
            Self::Convention => "ROLE_SOURCE_CONVENTION",
            Self::Declared => "ROLE_SOURCE_DECLARED",
            Self::Datasheet => "ROLE_SOURCE_DATASHEET",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for RoleSource {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "ROLE_SOURCE_UNSPECIFIED",
            "ROLE_SOURCE_CONVENTION",
            "ROLE_SOURCE_DECLARED",
            "ROLE_SOURCE_DATASHEET",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = RoleSource;

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
                    "ROLE_SOURCE_UNSPECIFIED" => Ok(RoleSource::Unspecified),
                    "ROLE_SOURCE_CONVENTION" => Ok(RoleSource::Convention),
                    "ROLE_SOURCE_DECLARED" => Ok(RoleSource::Declared),
                    "ROLE_SOURCE_DATASHEET" => Ok(RoleSource::Datasheet),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for Sheet {
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
        if !self.attributes.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.Sheet", len)?;
        if !self.id.is_empty() {
            struct_ser.serialize_field("id", &self.id)?;
        }
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
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
impl<'de> serde::Deserialize<'de> for Sheet {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "id",
            "name",
            "attributes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Id,
            Name,
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
            type Value = Sheet;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.Sheet")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Sheet, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut id__ = None;
                let mut name__ = None;
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
                Ok(Sheet {
                    id: id__.unwrap_or_default(),
                    name: name__.unwrap_or_default(),
                    attributes: attributes__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.Sheet", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Span {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.byte_offset != 0 {
            len += 1;
        }
        if self.byte_length != 0 {
            len += 1;
        }
        if self.line != 0 {
            len += 1;
        }
        if self.column != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.Span", len)?;
        if self.byte_offset != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("byteOffset", ToString::to_string(&self.byte_offset).as_str())?;
        }
        if self.byte_length != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("byteLength", ToString::to_string(&self.byte_length).as_str())?;
        }
        if self.line != 0 {
            struct_ser.serialize_field("line", &self.line)?;
        }
        if self.column != 0 {
            struct_ser.serialize_field("column", &self.column)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Span {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "byte_offset",
            "byteOffset",
            "byte_length",
            "byteLength",
            "line",
            "column",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            ByteOffset,
            ByteLength,
            Line,
            Column,
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
                            "byteOffset" | "byte_offset" => Ok(GeneratedField::ByteOffset),
                            "byteLength" | "byte_length" => Ok(GeneratedField::ByteLength),
                            "line" => Ok(GeneratedField::Line),
                            "column" => Ok(GeneratedField::Column),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Span;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.Span")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Span, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut byte_offset__ = None;
                let mut byte_length__ = None;
                let mut line__ = None;
                let mut column__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::ByteOffset => {
                            if byte_offset__.is_some() {
                                return Err(serde::de::Error::duplicate_field("byteOffset"));
                            }
                            byte_offset__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::ByteLength => {
                            if byte_length__.is_some() {
                                return Err(serde::de::Error::duplicate_field("byteLength"));
                            }
                            byte_length__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Line => {
                            if line__.is_some() {
                                return Err(serde::de::Error::duplicate_field("line"));
                            }
                            line__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Column => {
                            if column__.is_some() {
                                return Err(serde::de::Error::duplicate_field("column"));
                            }
                            column__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(Span {
                    byte_offset: byte_offset__.unwrap_or_default(),
                    byte_length: byte_length__.unwrap_or_default(),
                    line: line__.unwrap_or_default(),
                    column: column__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.Span", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for UnannotatedComponent {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.ref_des.is_empty() {
            len += 1;
        }
        if !self.instances.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.UnannotatedComponent", len)?;
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if !self.instances.is_empty() {
            struct_ser.serialize_field("instances", &self.instances)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for UnannotatedComponent {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "ref_des",
            "refDes",
            "instances",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            RefDes,
            Instances,
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
                            "refDes" | "ref_des" => Ok(GeneratedField::RefDes),
                            "instances" => Ok(GeneratedField::Instances),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = UnannotatedComponent;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.UnannotatedComponent")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<UnannotatedComponent, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut ref_des__ = None;
                let mut instances__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Instances => {
                            if instances__.is_some() {
                                return Err(serde::de::Error::duplicate_field("instances"));
                            }
                            instances__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(UnannotatedComponent {
                    ref_des: ref_des__.unwrap_or_default(),
                    instances: instances__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.UnannotatedComponent", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for UnexpandedHierarchy {
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
        if !self.kind.is_empty() {
            len += 1;
        }
        if self.instance_count != 0 {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        if self.reason != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.UnexpandedHierarchy", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if self.instance_count != 0 {
            struct_ser.serialize_field("instanceCount", &self.instance_count)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        if self.reason != 0 {
            let v = UnexpandedReason::try_from(self.reason)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.reason)))?;
            struct_ser.serialize_field("reason", &v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for UnexpandedHierarchy {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "kind",
            "instance_count",
            "instanceCount",
            "prov",
            "reason",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Kind,
            InstanceCount,
            Prov,
            Reason,
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
                            "kind" => Ok(GeneratedField::Kind),
                            "instanceCount" | "instance_count" => Ok(GeneratedField::InstanceCount),
                            "prov" => Ok(GeneratedField::Prov),
                            "reason" => Ok(GeneratedField::Reason),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = UnexpandedHierarchy;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.UnexpandedHierarchy")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<UnexpandedHierarchy, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut kind__ = None;
                let mut instance_count__ = None;
                let mut prov__ = None;
                let mut reason__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::InstanceCount => {
                            if instance_count__.is_some() {
                                return Err(serde::de::Error::duplicate_field("instanceCount"));
                            }
                            instance_count__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                        GeneratedField::Reason => {
                            if reason__.is_some() {
                                return Err(serde::de::Error::duplicate_field("reason"));
                            }
                            reason__ = Some(map_.next_value::<UnexpandedReason>()? as i32);
                        }
                    }
                }
                Ok(UnexpandedHierarchy {
                    name: name__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    instance_count: instance_count__.unwrap_or_default(),
                    prov: prov__,
                    reason: reason__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.UnexpandedHierarchy", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for UnexpandedReason {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "UNEXPANDED_REASON_UNSPECIFIED",
            Self::NotWalked => "UNEXPANDED_REASON_NOT_WALKED",
            Self::Cycle => "UNEXPANDED_REASON_CYCLE",
            Self::FileMissing => "UNEXPANDED_REASON_FILE_MISSING",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for UnexpandedReason {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "UNEXPANDED_REASON_UNSPECIFIED",
            "UNEXPANDED_REASON_NOT_WALKED",
            "UNEXPANDED_REASON_CYCLE",
            "UNEXPANDED_REASON_FILE_MISSING",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = UnexpandedReason;

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
                    "UNEXPANDED_REASON_UNSPECIFIED" => Ok(UnexpandedReason::Unspecified),
                    "UNEXPANDED_REASON_NOT_WALKED" => Ok(UnexpandedReason::NotWalked),
                    "UNEXPANDED_REASON_CYCLE" => Ok(UnexpandedReason::Cycle),
                    "UNEXPANDED_REASON_FILE_MISSING" => Ok(UnexpandedReason::FileMissing),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for UnresolvedSymbol {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.symref.is_empty() {
            len += 1;
        }
        if !self.kind.is_empty() {
            len += 1;
        }
        if !self.ref_des.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.ir.UnresolvedSymbol", len)?;
        if !self.symref.is_empty() {
            struct_ser.serialize_field("symref", &self.symref)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for UnresolvedSymbol {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "symref",
            "kind",
            "ref_des",
            "refDes",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Symref,
            Kind,
            RefDes,
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
                            "symref" => Ok(GeneratedField::Symref),
                            "kind" => Ok(GeneratedField::Kind),
                            "refDes" | "ref_des" => Ok(GeneratedField::RefDes),
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
            type Value = UnresolvedSymbol;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.ir.UnresolvedSymbol")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<UnresolvedSymbol, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut symref__ = None;
                let mut kind__ = None;
                let mut ref_des__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Symref => {
                            if symref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("symref"));
                            }
                            symref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(UnresolvedSymbol {
                    symref: symref__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    ref_des: ref_des__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.ir.UnresolvedSymbol", FIELDS, GeneratedVisitor)
    }
}
