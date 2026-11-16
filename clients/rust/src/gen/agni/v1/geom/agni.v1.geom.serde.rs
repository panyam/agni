// @generated
impl serde::Serialize for Asset {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.kind != 0 {
            len += 1;
        }
        if !self.id.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        if self.placeholder {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Asset", len)?;
        if self.kind != 0 {
            let v = asset::Kind::try_from(self.kind)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.kind)))?;
            struct_ser.serialize_field("kind", &v)?;
        }
        if !self.id.is_empty() {
            struct_ser.serialize_field("id", &self.id)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        if self.placeholder {
            struct_ser.serialize_field("placeholder", &self.placeholder)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Asset {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "kind",
            "id",
            "prov",
            "placeholder",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Kind,
            Id,
            Prov,
            Placeholder,
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
                            "id" => Ok(GeneratedField::Id),
                            "prov" => Ok(GeneratedField::Prov),
                            "placeholder" => Ok(GeneratedField::Placeholder),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Asset;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.Asset")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Asset, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut kind__ = None;
                let mut id__ = None;
                let mut prov__ = None;
                let mut placeholder__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value::<asset::Kind>()? as i32);
                        }
                        GeneratedField::Id => {
                            if id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("id"));
                            }
                            id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                        GeneratedField::Placeholder => {
                            if placeholder__.is_some() {
                                return Err(serde::de::Error::duplicate_field("placeholder"));
                            }
                            placeholder__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Asset {
                    kind: kind__.unwrap_or_default(),
                    id: id__.unwrap_or_default(),
                    prov: prov__,
                    placeholder: placeholder__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Asset", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for asset::Kind {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "KIND_UNSPECIFIED",
            Self::Symbol => "KIND_SYMBOL",
            Self::Image => "KIND_IMAGE",
            Self::Group => "KIND_GROUP",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for asset::Kind {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "KIND_UNSPECIFIED",
            "KIND_SYMBOL",
            "KIND_IMAGE",
            "KIND_GROUP",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = asset::Kind;

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
                    "KIND_UNSPECIFIED" => Ok(asset::Kind::Unspecified),
                    "KIND_SYMBOL" => Ok(asset::Kind::Symbol),
                    "KIND_IMAGE" => Ok(asset::Kind::Image),
                    "KIND_GROUP" => Ok(asset::Kind::Group),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for BBox {
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
        if self.max.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.BBox", len)?;
        if let Some(v) = self.min.as_ref() {
            struct_ser.serialize_field("min", v)?;
        }
        if let Some(v) = self.max.as_ref() {
            struct_ser.serialize_field("max", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BBox {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "min",
            "max",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Min,
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
            type Value = BBox;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.BBox")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BBox, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut min__ = None;
                let mut max__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Min => {
                            if min__.is_some() {
                                return Err(serde::de::Error::duplicate_field("min"));
                            }
                            min__ = map_.next_value()?;
                        }
                        GeneratedField::Max => {
                            if max__.is_some() {
                                return Err(serde::de::Error::duplicate_field("max"));
                            }
                            max__ = map_.next_value()?;
                        }
                    }
                }
                Ok(BBox {
                    min: min__,
                    max: max__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.BBox", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BackAnnotatedPlacement {
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
        if !self.drawn_ref_des.is_empty() {
            len += 1;
        }
        if !self.sheet_id.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.BackAnnotatedPlacement", len)?;
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if !self.drawn_ref_des.is_empty() {
            struct_ser.serialize_field("drawnRefDes", &self.drawn_ref_des)?;
        }
        if !self.sheet_id.is_empty() {
            struct_ser.serialize_field("sheetId", &self.sheet_id)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BackAnnotatedPlacement {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "ref_des",
            "refDes",
            "drawn_ref_des",
            "drawnRefDes",
            "sheet_id",
            "sheetId",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            RefDes,
            DrawnRefDes,
            SheetId,
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
                            "drawnRefDes" | "drawn_ref_des" => Ok(GeneratedField::DrawnRefDes),
                            "sheetId" | "sheet_id" => Ok(GeneratedField::SheetId),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BackAnnotatedPlacement;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.BackAnnotatedPlacement")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BackAnnotatedPlacement, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut ref_des__ = None;
                let mut drawn_ref_des__ = None;
                let mut sheet_id__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::DrawnRefDes => {
                            if drawn_ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("drawnRefDes"));
                            }
                            drawn_ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::SheetId => {
                            if sheet_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sheetId"));
                            }
                            sheet_id__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(BackAnnotatedPlacement {
                    ref_des: ref_des__.unwrap_or_default(),
                    drawn_ref_des: drawn_ref_des__.unwrap_or_default(),
                    sheet_id: sheet_id__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.BackAnnotatedPlacement", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BoardDrillSpan {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.layer_from.is_empty() {
            len += 1;
        }
        if !self.layer_to.is_empty() {
            len += 1;
        }
        if !self.xy.is_empty() {
            len += 1;
        }
        if !self.diameter.is_empty() {
            len += 1;
        }
        if !self.net.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.BoardDrillSpan", len)?;
        if !self.layer_from.is_empty() {
            struct_ser.serialize_field("layerFrom", &self.layer_from)?;
        }
        if !self.layer_to.is_empty() {
            struct_ser.serialize_field("layerTo", &self.layer_to)?;
        }
        if !self.xy.is_empty() {
            struct_ser.serialize_field("xy", &self.xy)?;
        }
        if !self.diameter.is_empty() {
            struct_ser.serialize_field("diameter", &self.diameter)?;
        }
        if !self.net.is_empty() {
            struct_ser.serialize_field("net", &self.net)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BoardDrillSpan {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "layer_from",
            "layerFrom",
            "layer_to",
            "layerTo",
            "xy",
            "diameter",
            "net",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            LayerFrom,
            LayerTo,
            Xy,
            Diameter,
            Net,
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
                            "layerFrom" | "layer_from" => Ok(GeneratedField::LayerFrom),
                            "layerTo" | "layer_to" => Ok(GeneratedField::LayerTo),
                            "xy" => Ok(GeneratedField::Xy),
                            "diameter" => Ok(GeneratedField::Diameter),
                            "net" => Ok(GeneratedField::Net),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BoardDrillSpan;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.BoardDrillSpan")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BoardDrillSpan, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut layer_from__ = None;
                let mut layer_to__ = None;
                let mut xy__ = None;
                let mut diameter__ = None;
                let mut net__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::LayerFrom => {
                            if layer_from__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layerFrom"));
                            }
                            layer_from__ = Some(map_.next_value()?);
                        }
                        GeneratedField::LayerTo => {
                            if layer_to__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layerTo"));
                            }
                            layer_to__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Xy => {
                            if xy__.is_some() {
                                return Err(serde::de::Error::duplicate_field("xy"));
                            }
                            xy__ = 
                                Some(map_.next_value::<Vec<::pbjson::private::NumberDeserialize<_>>>()?
                                    .into_iter().map(|x| x.0).collect())
                            ;
                        }
                        GeneratedField::Diameter => {
                            if diameter__.is_some() {
                                return Err(serde::de::Error::duplicate_field("diameter"));
                            }
                            diameter__ = 
                                Some(map_.next_value::<Vec<::pbjson::private::NumberDeserialize<_>>>()?
                                    .into_iter().map(|x| x.0).collect())
                            ;
                        }
                        GeneratedField::Net => {
                            if net__.is_some() {
                                return Err(serde::de::Error::duplicate_field("net"));
                            }
                            net__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(BoardDrillSpan {
                    layer_from: layer_from__.unwrap_or_default(),
                    layer_to: layer_to__.unwrap_or_default(),
                    xy: xy__.unwrap_or_default(),
                    diameter: diameter__.unwrap_or_default(),
                    net: net__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.BoardDrillSpan", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BoardGeometry {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.design_ref.is_empty() {
            len += 1;
        }
        if self.unit_nm != 0 {
            len += 1;
        }
        if !self.layers.is_empty() {
            len += 1;
        }
        if self.outline.is_some() {
            len += 1;
        }
        if !self.placements.is_empty() {
            len += 1;
        }
        if !self.nets.is_empty() {
            len += 1;
        }
        if !self.zones.is_empty() {
            len += 1;
        }
        if !self.texts.is_empty() {
            len += 1;
        }
        if !self.graphics.is_empty() {
            len += 1;
        }
        if !self.padstacks.is_empty() {
            len += 1;
        }
        if self.stackup.is_some() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.BoardGeometry", len)?;
        if !self.design_ref.is_empty() {
            struct_ser.serialize_field("designRef", &self.design_ref)?;
        }
        if self.unit_nm != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("unitNm", ToString::to_string(&self.unit_nm).as_str())?;
        }
        if !self.layers.is_empty() {
            struct_ser.serialize_field("layers", &self.layers)?;
        }
        if let Some(v) = self.outline.as_ref() {
            struct_ser.serialize_field("outline", v)?;
        }
        if !self.placements.is_empty() {
            struct_ser.serialize_field("placements", &self.placements)?;
        }
        if !self.nets.is_empty() {
            struct_ser.serialize_field("nets", &self.nets)?;
        }
        if !self.zones.is_empty() {
            struct_ser.serialize_field("zones", &self.zones)?;
        }
        if !self.texts.is_empty() {
            struct_ser.serialize_field("texts", &self.texts)?;
        }
        if !self.graphics.is_empty() {
            struct_ser.serialize_field("graphics", &self.graphics)?;
        }
        if !self.padstacks.is_empty() {
            struct_ser.serialize_field("padstacks", &self.padstacks)?;
        }
        if let Some(v) = self.stackup.as_ref() {
            struct_ser.serialize_field("stackup", v)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BoardGeometry {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "design_ref",
            "designRef",
            "unit_nm",
            "unitNm",
            "layers",
            "outline",
            "placements",
            "nets",
            "zones",
            "texts",
            "graphics",
            "padstacks",
            "stackup",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            DesignRef,
            UnitNm,
            Layers,
            Outline,
            Placements,
            Nets,
            Zones,
            Texts,
            Graphics,
            Padstacks,
            Stackup,
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
                            "designRef" | "design_ref" => Ok(GeneratedField::DesignRef),
                            "unitNm" | "unit_nm" => Ok(GeneratedField::UnitNm),
                            "layers" => Ok(GeneratedField::Layers),
                            "outline" => Ok(GeneratedField::Outline),
                            "placements" => Ok(GeneratedField::Placements),
                            "nets" => Ok(GeneratedField::Nets),
                            "zones" => Ok(GeneratedField::Zones),
                            "texts" => Ok(GeneratedField::Texts),
                            "graphics" => Ok(GeneratedField::Graphics),
                            "padstacks" => Ok(GeneratedField::Padstacks),
                            "stackup" => Ok(GeneratedField::Stackup),
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
            type Value = BoardGeometry;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.BoardGeometry")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BoardGeometry, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut design_ref__ = None;
                let mut unit_nm__ = None;
                let mut layers__ = None;
                let mut outline__ = None;
                let mut placements__ = None;
                let mut nets__ = None;
                let mut zones__ = None;
                let mut texts__ = None;
                let mut graphics__ = None;
                let mut padstacks__ = None;
                let mut stackup__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::DesignRef => {
                            if design_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("designRef"));
                            }
                            design_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::UnitNm => {
                            if unit_nm__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unitNm"));
                            }
                            unit_nm__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Layers => {
                            if layers__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layers"));
                            }
                            layers__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Outline => {
                            if outline__.is_some() {
                                return Err(serde::de::Error::duplicate_field("outline"));
                            }
                            outline__ = map_.next_value()?;
                        }
                        GeneratedField::Placements => {
                            if placements__.is_some() {
                                return Err(serde::de::Error::duplicate_field("placements"));
                            }
                            placements__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Nets => {
                            if nets__.is_some() {
                                return Err(serde::de::Error::duplicate_field("nets"));
                            }
                            nets__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Zones => {
                            if zones__.is_some() {
                                return Err(serde::de::Error::duplicate_field("zones"));
                            }
                            zones__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Texts => {
                            if texts__.is_some() {
                                return Err(serde::de::Error::duplicate_field("texts"));
                            }
                            texts__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Graphics => {
                            if graphics__.is_some() {
                                return Err(serde::de::Error::duplicate_field("graphics"));
                            }
                            graphics__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Padstacks => {
                            if padstacks__.is_some() {
                                return Err(serde::de::Error::duplicate_field("padstacks"));
                            }
                            padstacks__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Stackup => {
                            if stackup__.is_some() {
                                return Err(serde::de::Error::duplicate_field("stackup"));
                            }
                            stackup__ = map_.next_value()?;
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(BoardGeometry {
                    design_ref: design_ref__.unwrap_or_default(),
                    unit_nm: unit_nm__.unwrap_or_default(),
                    layers: layers__.unwrap_or_default(),
                    outline: outline__,
                    placements: placements__.unwrap_or_default(),
                    nets: nets__.unwrap_or_default(),
                    zones: zones__.unwrap_or_default(),
                    texts: texts__.unwrap_or_default(),
                    graphics: graphics__.unwrap_or_default(),
                    padstacks: padstacks__.unwrap_or_default(),
                    stackup: stackup__,
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.BoardGeometry", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BoardGraphic {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.shape.is_some() {
            len += 1;
        }
        if !self.layer.is_empty() {
            len += 1;
        }
        if !self.ref_des.is_empty() {
            len += 1;
        }
        if self.width != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.BoardGraphic", len)?;
        if let Some(v) = self.shape.as_ref() {
            struct_ser.serialize_field("shape", v)?;
        }
        if !self.layer.is_empty() {
            struct_ser.serialize_field("layer", &self.layer)?;
        }
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if self.width != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("width", ToString::to_string(&self.width).as_str())?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BoardGraphic {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "shape",
            "layer",
            "ref_des",
            "refDes",
            "width",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Shape,
            Layer,
            RefDes,
            Width,
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
                            "shape" => Ok(GeneratedField::Shape),
                            "layer" => Ok(GeneratedField::Layer),
                            "refDes" | "ref_des" => Ok(GeneratedField::RefDes),
                            "width" => Ok(GeneratedField::Width),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BoardGraphic;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.BoardGraphic")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BoardGraphic, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut shape__ = None;
                let mut layer__ = None;
                let mut ref_des__ = None;
                let mut width__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Shape => {
                            if shape__.is_some() {
                                return Err(serde::de::Error::duplicate_field("shape"));
                            }
                            shape__ = map_.next_value()?;
                        }
                        GeneratedField::Layer => {
                            if layer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layer"));
                            }
                            layer__ = Some(map_.next_value()?);
                        }
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Width => {
                            if width__.is_some() {
                                return Err(serde::de::Error::duplicate_field("width"));
                            }
                            width__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(BoardGraphic {
                    shape: shape__,
                    layer: layer__.unwrap_or_default(),
                    ref_des: ref_des__.unwrap_or_default(),
                    width: width__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.BoardGraphic", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BoardLayer {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.number != 0 {
            len += 1;
        }
        if !self.name.is_empty() {
            len += 1;
        }
        if !self.kind.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.BoardLayer", len)?;
        if self.number != 0 {
            struct_ser.serialize_field("number", &self.number)?;
        }
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BoardLayer {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "number",
            "name",
            "kind",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Number,
            Name,
            Kind,
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
                            "number" => Ok(GeneratedField::Number),
                            "name" => Ok(GeneratedField::Name),
                            "kind" => Ok(GeneratedField::Kind),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BoardLayer;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.BoardLayer")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BoardLayer, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut number__ = None;
                let mut name__ = None;
                let mut kind__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Number => {
                            if number__.is_some() {
                                return Err(serde::de::Error::duplicate_field("number"));
                            }
                            number__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
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
                    }
                }
                Ok(BoardLayer {
                    number: number__.unwrap_or_default(),
                    name: name__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.BoardLayer", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BoardLayerRow {
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
        if !self.side.is_empty() {
            len += 1;
        }
        if !self.color.is_empty() {
            len += 1;
        }
        if !self.svg_selector.is_empty() {
            len += 1;
        }
        if !self.groups.is_empty() {
            len += 1;
        }
        if !self.thickness.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.BoardLayerRow", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if !self.side.is_empty() {
            struct_ser.serialize_field("side", &self.side)?;
        }
        if !self.color.is_empty() {
            struct_ser.serialize_field("color", &self.color)?;
        }
        if !self.svg_selector.is_empty() {
            struct_ser.serialize_field("svgSelector", &self.svg_selector)?;
        }
        if !self.groups.is_empty() {
            struct_ser.serialize_field("groups", &self.groups)?;
        }
        if !self.thickness.is_empty() {
            struct_ser.serialize_field("thickness", &self.thickness)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BoardLayerRow {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "kind",
            "side",
            "color",
            "svg_selector",
            "svgSelector",
            "groups",
            "thickness",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Kind,
            Side,
            Color,
            SvgSelector,
            Groups,
            Thickness,
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
                            "side" => Ok(GeneratedField::Side),
                            "color" => Ok(GeneratedField::Color),
                            "svgSelector" | "svg_selector" => Ok(GeneratedField::SvgSelector),
                            "groups" => Ok(GeneratedField::Groups),
                            "thickness" => Ok(GeneratedField::Thickness),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BoardLayerRow;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.BoardLayerRow")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BoardLayerRow, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut kind__ = None;
                let mut side__ = None;
                let mut color__ = None;
                let mut svg_selector__ = None;
                let mut groups__ = None;
                let mut thickness__ = None;
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
                        GeneratedField::Side => {
                            if side__.is_some() {
                                return Err(serde::de::Error::duplicate_field("side"));
                            }
                            side__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Color => {
                            if color__.is_some() {
                                return Err(serde::de::Error::duplicate_field("color"));
                            }
                            color__ = Some(map_.next_value()?);
                        }
                        GeneratedField::SvgSelector => {
                            if svg_selector__.is_some() {
                                return Err(serde::de::Error::duplicate_field("svgSelector"));
                            }
                            svg_selector__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Groups => {
                            if groups__.is_some() {
                                return Err(serde::de::Error::duplicate_field("groups"));
                            }
                            groups__ = 
                                Some(map_.next_value::<Vec<::pbjson::private::NumberDeserialize<_>>>()?
                                    .into_iter().map(|x| x.0).collect())
                            ;
                        }
                        GeneratedField::Thickness => {
                            if thickness__.is_some() {
                                return Err(serde::de::Error::duplicate_field("thickness"));
                            }
                            thickness__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(BoardLayerRow {
                    name: name__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    side: side__.unwrap_or_default(),
                    color: color__.unwrap_or_default(),
                    svg_selector: svg_selector__.unwrap_or_default(),
                    groups: groups__.unwrap_or_default(),
                    thickness: thickness__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.BoardLayerRow", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BoardLook {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.mask.is_empty() {
            len += 1;
        }
        if !self.mask_over_copper.is_empty() {
            len += 1;
        }
        if !self.pad_finish.is_empty() {
            len += 1;
        }
        if !self.silk.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.BoardLook", len)?;
        if !self.mask.is_empty() {
            struct_ser.serialize_field("mask", &self.mask)?;
        }
        if !self.mask_over_copper.is_empty() {
            struct_ser.serialize_field("maskOverCopper", &self.mask_over_copper)?;
        }
        if !self.pad_finish.is_empty() {
            struct_ser.serialize_field("padFinish", &self.pad_finish)?;
        }
        if !self.silk.is_empty() {
            struct_ser.serialize_field("silk", &self.silk)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BoardLook {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "mask",
            "mask_over_copper",
            "maskOverCopper",
            "pad_finish",
            "padFinish",
            "silk",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Mask,
            MaskOverCopper,
            PadFinish,
            Silk,
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
                            "mask" => Ok(GeneratedField::Mask),
                            "maskOverCopper" | "mask_over_copper" => Ok(GeneratedField::MaskOverCopper),
                            "padFinish" | "pad_finish" => Ok(GeneratedField::PadFinish),
                            "silk" => Ok(GeneratedField::Silk),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BoardLook;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.BoardLook")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BoardLook, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut mask__ = None;
                let mut mask_over_copper__ = None;
                let mut pad_finish__ = None;
                let mut silk__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Mask => {
                            if mask__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mask"));
                            }
                            mask__ = Some(map_.next_value()?);
                        }
                        GeneratedField::MaskOverCopper => {
                            if mask_over_copper__.is_some() {
                                return Err(serde::de::Error::duplicate_field("maskOverCopper"));
                            }
                            mask_over_copper__ = Some(map_.next_value()?);
                        }
                        GeneratedField::PadFinish => {
                            if pad_finish__.is_some() {
                                return Err(serde::de::Error::duplicate_field("padFinish"));
                            }
                            pad_finish__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Silk => {
                            if silk__.is_some() {
                                return Err(serde::de::Error::duplicate_field("silk"));
                            }
                            silk__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(BoardLook {
                    mask: mask__.unwrap_or_default(),
                    mask_over_copper: mask_over_copper__.unwrap_or_default(),
                    pad_finish: pad_finish__.unwrap_or_default(),
                    silk: silk__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.BoardLook", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BoardOutline {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.paths.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.BoardOutline", len)?;
        if !self.paths.is_empty() {
            struct_ser.serialize_field("paths", &self.paths)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BoardOutline {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "paths",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Paths,
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
                            "paths" => Ok(GeneratedField::Paths),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BoardOutline;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.BoardOutline")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BoardOutline, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut paths__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Paths => {
                            if paths__.is_some() {
                                return Err(serde::de::Error::duplicate_field("paths"));
                            }
                            paths__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(BoardOutline {
                    paths: paths__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.BoardOutline", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BoardPartBody {
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
        if !self.side.is_empty() {
            len += 1;
        }
        if !self.outline.is_empty() {
            len += 1;
        }
        if self.height_mm.is_some() {
            len += 1;
        }
        if !self.part_class.is_empty() {
            len += 1;
        }
        if !self.color.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.BoardPartBody", len)?;
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if !self.side.is_empty() {
            struct_ser.serialize_field("side", &self.side)?;
        }
        if !self.outline.is_empty() {
            struct_ser.serialize_field("outline", &self.outline)?;
        }
        if let Some(v) = self.height_mm.as_ref() {
            struct_ser.serialize_field("heightMm", v)?;
        }
        if !self.part_class.is_empty() {
            struct_ser.serialize_field("partClass", &self.part_class)?;
        }
        if !self.color.is_empty() {
            struct_ser.serialize_field("color", &self.color)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BoardPartBody {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "ref_des",
            "refDes",
            "side",
            "outline",
            "height_mm",
            "heightMm",
            "part_class",
            "partClass",
            "color",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            RefDes,
            Side,
            Outline,
            HeightMm,
            PartClass,
            Color,
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
                            "side" => Ok(GeneratedField::Side),
                            "outline" => Ok(GeneratedField::Outline),
                            "heightMm" | "height_mm" => Ok(GeneratedField::HeightMm),
                            "partClass" | "part_class" => Ok(GeneratedField::PartClass),
                            "color" => Ok(GeneratedField::Color),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BoardPartBody;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.BoardPartBody")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BoardPartBody, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut ref_des__ = None;
                let mut side__ = None;
                let mut outline__ = None;
                let mut height_mm__ = None;
                let mut part_class__ = None;
                let mut color__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Side => {
                            if side__.is_some() {
                                return Err(serde::de::Error::duplicate_field("side"));
                            }
                            side__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Outline => {
                            if outline__.is_some() {
                                return Err(serde::de::Error::duplicate_field("outline"));
                            }
                            outline__ = 
                                Some(map_.next_value::<Vec<::pbjson::private::NumberDeserialize<_>>>()?
                                    .into_iter().map(|x| x.0).collect())
                            ;
                        }
                        GeneratedField::HeightMm => {
                            if height_mm__.is_some() {
                                return Err(serde::de::Error::duplicate_field("heightMm"));
                            }
                            height_mm__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                        GeneratedField::PartClass => {
                            if part_class__.is_some() {
                                return Err(serde::de::Error::duplicate_field("partClass"));
                            }
                            part_class__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Color => {
                            if color__.is_some() {
                                return Err(serde::de::Error::duplicate_field("color"));
                            }
                            color__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(BoardPartBody {
                    ref_des: ref_des__.unwrap_or_default(),
                    side: side__.unwrap_or_default(),
                    outline: outline__.unwrap_or_default(),
                    height_mm: height_mm__,
                    part_class: part_class__.unwrap_or_default(),
                    color: color__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.BoardPartBody", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BoardStackRow {
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
        if !self.layer.is_empty() {
            len += 1;
        }
        if !self.thickness.is_empty() {
            len += 1;
        }
        if self.thickness_mm.is_some() {
            len += 1;
        }
        if !self.material.is_empty() {
            len += 1;
        }
        if !self.color.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.BoardStackRow", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if !self.layer.is_empty() {
            struct_ser.serialize_field("layer", &self.layer)?;
        }
        if !self.thickness.is_empty() {
            struct_ser.serialize_field("thickness", &self.thickness)?;
        }
        if let Some(v) = self.thickness_mm.as_ref() {
            struct_ser.serialize_field("thicknessMm", v)?;
        }
        if !self.material.is_empty() {
            struct_ser.serialize_field("material", &self.material)?;
        }
        if !self.color.is_empty() {
            struct_ser.serialize_field("color", &self.color)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BoardStackRow {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "kind",
            "layer",
            "thickness",
            "thickness_mm",
            "thicknessMm",
            "material",
            "color",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Kind,
            Layer,
            Thickness,
            ThicknessMm,
            Material,
            Color,
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
                            "layer" => Ok(GeneratedField::Layer),
                            "thickness" => Ok(GeneratedField::Thickness),
                            "thicknessMm" | "thickness_mm" => Ok(GeneratedField::ThicknessMm),
                            "material" => Ok(GeneratedField::Material),
                            "color" => Ok(GeneratedField::Color),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BoardStackRow;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.BoardStackRow")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BoardStackRow, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut kind__ = None;
                let mut layer__ = None;
                let mut thickness__ = None;
                let mut thickness_mm__ = None;
                let mut material__ = None;
                let mut color__ = None;
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
                        GeneratedField::Layer => {
                            if layer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layer"));
                            }
                            layer__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Thickness => {
                            if thickness__.is_some() {
                                return Err(serde::de::Error::duplicate_field("thickness"));
                            }
                            thickness__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ThicknessMm => {
                            if thickness_mm__.is_some() {
                                return Err(serde::de::Error::duplicate_field("thicknessMm"));
                            }
                            thickness_mm__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                        GeneratedField::Material => {
                            if material__.is_some() {
                                return Err(serde::de::Error::duplicate_field("material"));
                            }
                            material__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Color => {
                            if color__.is_some() {
                                return Err(serde::de::Error::duplicate_field("color"));
                            }
                            color__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(BoardStackRow {
                    name: name__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    layer: layer__.unwrap_or_default(),
                    thickness: thickness__.unwrap_or_default(),
                    thickness_mm: thickness_mm__,
                    material: material__.unwrap_or_default(),
                    color: color__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.BoardStackRow", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BoardStackView {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.rows.is_empty() {
            len += 1;
        }
        if !self.summary.is_empty() {
            len += 1;
        }
        if self.disagrees {
            len += 1;
        }
        if !self.drills.is_empty() {
            len += 1;
        }
        if !self.outline_tris.is_empty() {
            len += 1;
        }
        if self.units_per_mm != 0. {
            len += 1;
        }
        if !self.bodies.is_empty() {
            len += 1;
        }
        if !self.body_color.is_empty() {
            len += 1;
        }
        if !self.default_body_color.is_empty() {
            len += 1;
        }
        if self.board_look.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.BoardStackView", len)?;
        if !self.rows.is_empty() {
            struct_ser.serialize_field("rows", &self.rows)?;
        }
        if !self.summary.is_empty() {
            struct_ser.serialize_field("summary", &self.summary)?;
        }
        if self.disagrees {
            struct_ser.serialize_field("disagrees", &self.disagrees)?;
        }
        if !self.drills.is_empty() {
            struct_ser.serialize_field("drills", &self.drills)?;
        }
        if !self.outline_tris.is_empty() {
            struct_ser.serialize_field("outlineTris", &self.outline_tris)?;
        }
        if self.units_per_mm != 0. {
            struct_ser.serialize_field("unitsPerMm", &self.units_per_mm)?;
        }
        if !self.bodies.is_empty() {
            struct_ser.serialize_field("bodies", &self.bodies)?;
        }
        if !self.body_color.is_empty() {
            struct_ser.serialize_field("bodyColor", &self.body_color)?;
        }
        if !self.default_body_color.is_empty() {
            struct_ser.serialize_field("defaultBodyColor", &self.default_body_color)?;
        }
        if let Some(v) = self.board_look.as_ref() {
            struct_ser.serialize_field("boardLook", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BoardStackView {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "rows",
            "summary",
            "disagrees",
            "drills",
            "outline_tris",
            "outlineTris",
            "units_per_mm",
            "unitsPerMm",
            "bodies",
            "body_color",
            "bodyColor",
            "default_body_color",
            "defaultBodyColor",
            "board_look",
            "boardLook",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Rows,
            Summary,
            Disagrees,
            Drills,
            OutlineTris,
            UnitsPerMm,
            Bodies,
            BodyColor,
            DefaultBodyColor,
            BoardLook,
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
                            "rows" => Ok(GeneratedField::Rows),
                            "summary" => Ok(GeneratedField::Summary),
                            "disagrees" => Ok(GeneratedField::Disagrees),
                            "drills" => Ok(GeneratedField::Drills),
                            "outlineTris" | "outline_tris" => Ok(GeneratedField::OutlineTris),
                            "unitsPerMm" | "units_per_mm" => Ok(GeneratedField::UnitsPerMm),
                            "bodies" => Ok(GeneratedField::Bodies),
                            "bodyColor" | "body_color" => Ok(GeneratedField::BodyColor),
                            "defaultBodyColor" | "default_body_color" => Ok(GeneratedField::DefaultBodyColor),
                            "boardLook" | "board_look" => Ok(GeneratedField::BoardLook),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BoardStackView;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.BoardStackView")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BoardStackView, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut rows__ = None;
                let mut summary__ = None;
                let mut disagrees__ = None;
                let mut drills__ = None;
                let mut outline_tris__ = None;
                let mut units_per_mm__ = None;
                let mut bodies__ = None;
                let mut body_color__ = None;
                let mut default_body_color__ = None;
                let mut board_look__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Rows => {
                            if rows__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rows"));
                            }
                            rows__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Summary => {
                            if summary__.is_some() {
                                return Err(serde::de::Error::duplicate_field("summary"));
                            }
                            summary__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Disagrees => {
                            if disagrees__.is_some() {
                                return Err(serde::de::Error::duplicate_field("disagrees"));
                            }
                            disagrees__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Drills => {
                            if drills__.is_some() {
                                return Err(serde::de::Error::duplicate_field("drills"));
                            }
                            drills__ = Some(map_.next_value()?);
                        }
                        GeneratedField::OutlineTris => {
                            if outline_tris__.is_some() {
                                return Err(serde::de::Error::duplicate_field("outlineTris"));
                            }
                            outline_tris__ = 
                                Some(map_.next_value::<Vec<::pbjson::private::NumberDeserialize<_>>>()?
                                    .into_iter().map(|x| x.0).collect())
                            ;
                        }
                        GeneratedField::UnitsPerMm => {
                            if units_per_mm__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unitsPerMm"));
                            }
                            units_per_mm__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Bodies => {
                            if bodies__.is_some() {
                                return Err(serde::de::Error::duplicate_field("bodies"));
                            }
                            bodies__ = Some(map_.next_value()?);
                        }
                        GeneratedField::BodyColor => {
                            if body_color__.is_some() {
                                return Err(serde::de::Error::duplicate_field("bodyColor"));
                            }
                            body_color__ = Some(map_.next_value()?);
                        }
                        GeneratedField::DefaultBodyColor => {
                            if default_body_color__.is_some() {
                                return Err(serde::de::Error::duplicate_field("defaultBodyColor"));
                            }
                            default_body_color__ = Some(map_.next_value()?);
                        }
                        GeneratedField::BoardLook => {
                            if board_look__.is_some() {
                                return Err(serde::de::Error::duplicate_field("boardLook"));
                            }
                            board_look__ = map_.next_value()?;
                        }
                    }
                }
                Ok(BoardStackView {
                    rows: rows__.unwrap_or_default(),
                    summary: summary__.unwrap_or_default(),
                    disagrees: disagrees__.unwrap_or_default(),
                    drills: drills__.unwrap_or_default(),
                    outline_tris: outline_tris__.unwrap_or_default(),
                    units_per_mm: units_per_mm__.unwrap_or_default(),
                    bodies: bodies__.unwrap_or_default(),
                    body_color: body_color__.unwrap_or_default(),
                    default_body_color: default_body_color__.unwrap_or_default(),
                    board_look: board_look__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.BoardStackView", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for BoardText {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.text.is_empty() {
            len += 1;
        }
        if self.at.is_some() {
            len += 1;
        }
        if self.rotation_deg != 0. {
            len += 1;
        }
        if self.height != 0 {
            len += 1;
        }
        if !self.layer.is_empty() {
            len += 1;
        }
        if self.mirror {
            len += 1;
        }
        if !self.justify.is_empty() {
            len += 1;
        }
        if !self.ref_des.is_empty() {
            len += 1;
        }
        if !self.kind.is_empty() {
            len += 1;
        }
        if !self.font.is_empty() {
            len += 1;
        }
        if self.thickness != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.BoardText", len)?;
        if !self.text.is_empty() {
            struct_ser.serialize_field("text", &self.text)?;
        }
        if let Some(v) = self.at.as_ref() {
            struct_ser.serialize_field("at", v)?;
        }
        if self.rotation_deg != 0. {
            struct_ser.serialize_field("rotationDeg", &self.rotation_deg)?;
        }
        if self.height != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("height", ToString::to_string(&self.height).as_str())?;
        }
        if !self.layer.is_empty() {
            struct_ser.serialize_field("layer", &self.layer)?;
        }
        if self.mirror {
            struct_ser.serialize_field("mirror", &self.mirror)?;
        }
        if !self.justify.is_empty() {
            struct_ser.serialize_field("justify", &self.justify)?;
        }
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if !self.font.is_empty() {
            struct_ser.serialize_field("font", &self.font)?;
        }
        if self.thickness != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("thickness", ToString::to_string(&self.thickness).as_str())?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for BoardText {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "text",
            "at",
            "rotation_deg",
            "rotationDeg",
            "height",
            "layer",
            "mirror",
            "justify",
            "ref_des",
            "refDes",
            "kind",
            "font",
            "thickness",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Text,
            At,
            RotationDeg,
            Height,
            Layer,
            Mirror,
            Justify,
            RefDes,
            Kind,
            Font,
            Thickness,
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
                            "text" => Ok(GeneratedField::Text),
                            "at" => Ok(GeneratedField::At),
                            "rotationDeg" | "rotation_deg" => Ok(GeneratedField::RotationDeg),
                            "height" => Ok(GeneratedField::Height),
                            "layer" => Ok(GeneratedField::Layer),
                            "mirror" => Ok(GeneratedField::Mirror),
                            "justify" => Ok(GeneratedField::Justify),
                            "refDes" | "ref_des" => Ok(GeneratedField::RefDes),
                            "kind" => Ok(GeneratedField::Kind),
                            "font" => Ok(GeneratedField::Font),
                            "thickness" => Ok(GeneratedField::Thickness),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = BoardText;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.BoardText")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<BoardText, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut text__ = None;
                let mut at__ = None;
                let mut rotation_deg__ = None;
                let mut height__ = None;
                let mut layer__ = None;
                let mut mirror__ = None;
                let mut justify__ = None;
                let mut ref_des__ = None;
                let mut kind__ = None;
                let mut font__ = None;
                let mut thickness__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Text => {
                            if text__.is_some() {
                                return Err(serde::de::Error::duplicate_field("text"));
                            }
                            text__ = Some(map_.next_value()?);
                        }
                        GeneratedField::At => {
                            if at__.is_some() {
                                return Err(serde::de::Error::duplicate_field("at"));
                            }
                            at__ = map_.next_value()?;
                        }
                        GeneratedField::RotationDeg => {
                            if rotation_deg__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rotationDeg"));
                            }
                            rotation_deg__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Height => {
                            if height__.is_some() {
                                return Err(serde::de::Error::duplicate_field("height"));
                            }
                            height__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Layer => {
                            if layer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layer"));
                            }
                            layer__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Mirror => {
                            if mirror__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mirror"));
                            }
                            mirror__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Justify => {
                            if justify__.is_some() {
                                return Err(serde::de::Error::duplicate_field("justify"));
                            }
                            justify__ = Some(map_.next_value()?);
                        }
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Font => {
                            if font__.is_some() {
                                return Err(serde::de::Error::duplicate_field("font"));
                            }
                            font__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Thickness => {
                            if thickness__.is_some() {
                                return Err(serde::de::Error::duplicate_field("thickness"));
                            }
                            thickness__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(BoardText {
                    text: text__.unwrap_or_default(),
                    at: at__,
                    rotation_deg: rotation_deg__.unwrap_or_default(),
                    height: height__.unwrap_or_default(),
                    layer: layer__.unwrap_or_default(),
                    mirror: mirror__.unwrap_or_default(),
                    justify: justify__.unwrap_or_default(),
                    ref_des: ref_des__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    font: font__.unwrap_or_default(),
                    thickness: thickness__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.BoardText", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ComponentPlacement {
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
        if self.at.is_some() {
            len += 1;
        }
        if self.rotation_deg != 0. {
            len += 1;
        }
        if !self.layer.is_empty() {
            len += 1;
        }
        if !self.pads.is_empty() {
            len += 1;
        }
        if self.mirror {
            len += 1;
        }
        if self.height.is_some() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.ComponentPlacement", len)?;
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if let Some(v) = self.at.as_ref() {
            struct_ser.serialize_field("at", v)?;
        }
        if self.rotation_deg != 0. {
            struct_ser.serialize_field("rotationDeg", &self.rotation_deg)?;
        }
        if !self.layer.is_empty() {
            struct_ser.serialize_field("layer", &self.layer)?;
        }
        if !self.pads.is_empty() {
            struct_ser.serialize_field("pads", &self.pads)?;
        }
        if self.mirror {
            struct_ser.serialize_field("mirror", &self.mirror)?;
        }
        if let Some(v) = self.height.as_ref() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("height", ToString::to_string(&v).as_str())?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ComponentPlacement {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "ref_des",
            "refDes",
            "at",
            "rotation_deg",
            "rotationDeg",
            "layer",
            "pads",
            "mirror",
            "height",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            RefDes,
            At,
            RotationDeg,
            Layer,
            Pads,
            Mirror,
            Height,
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
                            "at" => Ok(GeneratedField::At),
                            "rotationDeg" | "rotation_deg" => Ok(GeneratedField::RotationDeg),
                            "layer" => Ok(GeneratedField::Layer),
                            "pads" => Ok(GeneratedField::Pads),
                            "mirror" => Ok(GeneratedField::Mirror),
                            "height" => Ok(GeneratedField::Height),
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
            type Value = ComponentPlacement;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.ComponentPlacement")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ComponentPlacement, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut ref_des__ = None;
                let mut at__ = None;
                let mut rotation_deg__ = None;
                let mut layer__ = None;
                let mut pads__ = None;
                let mut mirror__ = None;
                let mut height__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::At => {
                            if at__.is_some() {
                                return Err(serde::de::Error::duplicate_field("at"));
                            }
                            at__ = map_.next_value()?;
                        }
                        GeneratedField::RotationDeg => {
                            if rotation_deg__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rotationDeg"));
                            }
                            rotation_deg__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Layer => {
                            if layer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layer"));
                            }
                            layer__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Pads => {
                            if pads__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pads"));
                            }
                            pads__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Mirror => {
                            if mirror__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mirror"));
                            }
                            mirror__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Height => {
                            if height__.is_some() {
                                return Err(serde::de::Error::duplicate_field("height"));
                            }
                            height__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
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
                Ok(ComponentPlacement {
                    ref_des: ref_des__.unwrap_or_default(),
                    at: at__,
                    rotation_deg: rotation_deg__.unwrap_or_default(),
                    layer: layer__.unwrap_or_default(),
                    pads: pads__.unwrap_or_default(),
                    mirror: mirror__.unwrap_or_default(),
                    height: height__,
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.ComponentPlacement", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for CopperShape {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.layer.is_empty() {
            len += 1;
        }
        if self.outline.is_some() {
            len += 1;
        }
        if !self.holes.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.CopperShape", len)?;
        if !self.layer.is_empty() {
            struct_ser.serialize_field("layer", &self.layer)?;
        }
        if let Some(v) = self.outline.as_ref() {
            struct_ser.serialize_field("outline", v)?;
        }
        if !self.holes.is_empty() {
            struct_ser.serialize_field("holes", &self.holes)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for CopperShape {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "layer",
            "outline",
            "holes",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Layer,
            Outline,
            Holes,
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
                            "layer" => Ok(GeneratedField::Layer),
                            "outline" => Ok(GeneratedField::Outline),
                            "holes" => Ok(GeneratedField::Holes),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = CopperShape;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.CopperShape")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<CopperShape, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut layer__ = None;
                let mut outline__ = None;
                let mut holes__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Layer => {
                            if layer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layer"));
                            }
                            layer__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Outline => {
                            if outline__.is_some() {
                                return Err(serde::de::Error::duplicate_field("outline"));
                            }
                            outline__ = map_.next_value()?;
                        }
                        GeneratedField::Holes => {
                            if holes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("holes"));
                            }
                            holes__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(CopperShape {
                    layer: layer__.unwrap_or_default(),
                    outline: outline__,
                    holes: holes__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.CopperShape", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Field {
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
        if !self.value.is_empty() {
            len += 1;
        }
        if self.origin.is_some() {
            len += 1;
        }
        if !self.justify.is_empty() {
            len += 1;
        }
        if self.height != 0 {
            len += 1;
        }
        if self.rotation_deg != 0 {
            len += 1;
        }
        if self.visible {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Field", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.value.is_empty() {
            struct_ser.serialize_field("value", &self.value)?;
        }
        if let Some(v) = self.origin.as_ref() {
            struct_ser.serialize_field("origin", v)?;
        }
        if !self.justify.is_empty() {
            struct_ser.serialize_field("justify", &self.justify)?;
        }
        if self.height != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("height", ToString::to_string(&self.height).as_str())?;
        }
        if self.rotation_deg != 0 {
            struct_ser.serialize_field("rotationDeg", &self.rotation_deg)?;
        }
        if self.visible {
            struct_ser.serialize_field("visible", &self.visible)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Field {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "value",
            "origin",
            "justify",
            "height",
            "rotation_deg",
            "rotationDeg",
            "visible",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Value,
            Origin,
            Justify,
            Height,
            RotationDeg,
            Visible,
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
                            "value" => Ok(GeneratedField::Value),
                            "origin" => Ok(GeneratedField::Origin),
                            "justify" => Ok(GeneratedField::Justify),
                            "height" => Ok(GeneratedField::Height),
                            "rotationDeg" | "rotation_deg" => Ok(GeneratedField::RotationDeg),
                            "visible" => Ok(GeneratedField::Visible),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Field;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.Field")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Field, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut value__ = None;
                let mut origin__ = None;
                let mut justify__ = None;
                let mut height__ = None;
                let mut rotation_deg__ = None;
                let mut visible__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Value => {
                            if value__.is_some() {
                                return Err(serde::de::Error::duplicate_field("value"));
                            }
                            value__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Origin => {
                            if origin__.is_some() {
                                return Err(serde::de::Error::duplicate_field("origin"));
                            }
                            origin__ = map_.next_value()?;
                        }
                        GeneratedField::Justify => {
                            if justify__.is_some() {
                                return Err(serde::de::Error::duplicate_field("justify"));
                            }
                            justify__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Height => {
                            if height__.is_some() {
                                return Err(serde::de::Error::duplicate_field("height"));
                            }
                            height__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::RotationDeg => {
                            if rotation_deg__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rotationDeg"));
                            }
                            rotation_deg__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Visible => {
                            if visible__.is_some() {
                                return Err(serde::de::Error::duplicate_field("visible"));
                            }
                            visible__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Field {
                    name: name__.unwrap_or_default(),
                    value: value__.unwrap_or_default(),
                    origin: origin__,
                    justify: justify__.unwrap_or_default(),
                    height: height__.unwrap_or_default(),
                    rotation_deg: rotation_deg__.unwrap_or_default(),
                    visible: visible__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Field", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for HighlightShape {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "HIGHLIGHT_SHAPE_UNSPECIFIED",
            Self::Outline => "HIGHLIGHT_SHAPE_OUTLINE",
            Self::BoundingRect => "HIGHLIGHT_SHAPE_BOUNDING_RECT",
            Self::BoundingCircle => "HIGHLIGHT_SHAPE_BOUNDING_CIRCLE",
            Self::Path => "HIGHLIGHT_SHAPE_PATH",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for HighlightShape {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "HIGHLIGHT_SHAPE_UNSPECIFIED",
            "HIGHLIGHT_SHAPE_OUTLINE",
            "HIGHLIGHT_SHAPE_BOUNDING_RECT",
            "HIGHLIGHT_SHAPE_BOUNDING_CIRCLE",
            "HIGHLIGHT_SHAPE_PATH",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = HighlightShape;

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
                    "HIGHLIGHT_SHAPE_UNSPECIFIED" => Ok(HighlightShape::Unspecified),
                    "HIGHLIGHT_SHAPE_OUTLINE" => Ok(HighlightShape::Outline),
                    "HIGHLIGHT_SHAPE_BOUNDING_RECT" => Ok(HighlightShape::BoundingRect),
                    "HIGHLIGHT_SHAPE_BOUNDING_CIRCLE" => Ok(HighlightShape::BoundingCircle),
                    "HIGHLIGHT_SHAPE_PATH" => Ok(HighlightShape::Path),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for HighlightSpec {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.components.is_empty() {
            len += 1;
        }
        if !self.nets.is_empty() {
            len += 1;
        }
        if !self.pins.is_empty() {
            len += 1;
        }
        if !self.net_ids.is_empty() {
            len += 1;
        }
        if !self.bus_ids.is_empty() {
            len += 1;
        }
        if !self.color.is_empty() {
            len += 1;
        }
        if self.alpha != 0. {
            len += 1;
        }
        if self.shape != 0 {
            len += 1;
        }
        if self.stroke_scale != 0. {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.HighlightSpec", len)?;
        if !self.components.is_empty() {
            struct_ser.serialize_field("components", &self.components)?;
        }
        if !self.nets.is_empty() {
            struct_ser.serialize_field("nets", &self.nets)?;
        }
        if !self.pins.is_empty() {
            struct_ser.serialize_field("pins", &self.pins)?;
        }
        if !self.net_ids.is_empty() {
            struct_ser.serialize_field("netIds", &self.net_ids)?;
        }
        if !self.bus_ids.is_empty() {
            struct_ser.serialize_field("busIds", &self.bus_ids)?;
        }
        if !self.color.is_empty() {
            struct_ser.serialize_field("color", &self.color)?;
        }
        if self.alpha != 0. {
            struct_ser.serialize_field("alpha", &self.alpha)?;
        }
        if self.shape != 0 {
            let v = HighlightShape::try_from(self.shape)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.shape)))?;
            struct_ser.serialize_field("shape", &v)?;
        }
        if self.stroke_scale != 0. {
            struct_ser.serialize_field("strokeScale", &self.stroke_scale)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for HighlightSpec {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "components",
            "nets",
            "pins",
            "net_ids",
            "netIds",
            "bus_ids",
            "busIds",
            "color",
            "alpha",
            "shape",
            "stroke_scale",
            "strokeScale",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Components,
            Nets,
            Pins,
            NetIds,
            BusIds,
            Color,
            Alpha,
            Shape,
            StrokeScale,
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
                            "components" => Ok(GeneratedField::Components),
                            "nets" => Ok(GeneratedField::Nets),
                            "pins" => Ok(GeneratedField::Pins),
                            "netIds" | "net_ids" => Ok(GeneratedField::NetIds),
                            "busIds" | "bus_ids" => Ok(GeneratedField::BusIds),
                            "color" => Ok(GeneratedField::Color),
                            "alpha" => Ok(GeneratedField::Alpha),
                            "shape" => Ok(GeneratedField::Shape),
                            "strokeScale" | "stroke_scale" => Ok(GeneratedField::StrokeScale),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = HighlightSpec;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.HighlightSpec")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<HighlightSpec, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut components__ = None;
                let mut nets__ = None;
                let mut pins__ = None;
                let mut net_ids__ = None;
                let mut bus_ids__ = None;
                let mut color__ = None;
                let mut alpha__ = None;
                let mut shape__ = None;
                let mut stroke_scale__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
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
                        GeneratedField::Pins => {
                            if pins__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pins"));
                            }
                            pins__ = Some(map_.next_value()?);
                        }
                        GeneratedField::NetIds => {
                            if net_ids__.is_some() {
                                return Err(serde::de::Error::duplicate_field("netIds"));
                            }
                            net_ids__ = Some(map_.next_value()?);
                        }
                        GeneratedField::BusIds => {
                            if bus_ids__.is_some() {
                                return Err(serde::de::Error::duplicate_field("busIds"));
                            }
                            bus_ids__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Color => {
                            if color__.is_some() {
                                return Err(serde::de::Error::duplicate_field("color"));
                            }
                            color__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Alpha => {
                            if alpha__.is_some() {
                                return Err(serde::de::Error::duplicate_field("alpha"));
                            }
                            alpha__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Shape => {
                            if shape__.is_some() {
                                return Err(serde::de::Error::duplicate_field("shape"));
                            }
                            shape__ = Some(map_.next_value::<HighlightShape>()? as i32);
                        }
                        GeneratedField::StrokeScale => {
                            if stroke_scale__.is_some() {
                                return Err(serde::de::Error::duplicate_field("strokeScale"));
                            }
                            stroke_scale__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(HighlightSpec {
                    components: components__.unwrap_or_default(),
                    nets: nets__.unwrap_or_default(),
                    pins: pins__.unwrap_or_default(),
                    net_ids: net_ids__.unwrap_or_default(),
                    bus_ids: bus_ids__.unwrap_or_default(),
                    color: color__.unwrap_or_default(),
                    alpha: alpha__.unwrap_or_default(),
                    shape: shape__.unwrap_or_default(),
                    stroke_scale: stroke_scale__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.HighlightSpec", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Image {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.bbox.is_some() {
            len += 1;
        }
        if !self.mime.is_empty() {
            len += 1;
        }
        if !self.data.is_empty() {
            len += 1;
        }
        if self.rotation_deg != 0 {
            len += 1;
        }
        if self.mirror {
            len += 1;
        }
        if self.asset.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Image", len)?;
        if let Some(v) = self.bbox.as_ref() {
            struct_ser.serialize_field("bbox", v)?;
        }
        if !self.mime.is_empty() {
            struct_ser.serialize_field("mime", &self.mime)?;
        }
        if !self.data.is_empty() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("data", pbjson::private::base64::encode(&self.data).as_str())?;
        }
        if self.rotation_deg != 0 {
            struct_ser.serialize_field("rotationDeg", &self.rotation_deg)?;
        }
        if self.mirror {
            struct_ser.serialize_field("mirror", &self.mirror)?;
        }
        if let Some(v) = self.asset.as_ref() {
            struct_ser.serialize_field("asset", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Image {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "bbox",
            "mime",
            "data",
            "rotation_deg",
            "rotationDeg",
            "mirror",
            "asset",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Bbox,
            Mime,
            Data,
            RotationDeg,
            Mirror,
            Asset,
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
                            "bbox" => Ok(GeneratedField::Bbox),
                            "mime" => Ok(GeneratedField::Mime),
                            "data" => Ok(GeneratedField::Data),
                            "rotationDeg" | "rotation_deg" => Ok(GeneratedField::RotationDeg),
                            "mirror" => Ok(GeneratedField::Mirror),
                            "asset" => Ok(GeneratedField::Asset),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Image;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.Image")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Image, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut bbox__ = None;
                let mut mime__ = None;
                let mut data__ = None;
                let mut rotation_deg__ = None;
                let mut mirror__ = None;
                let mut asset__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Bbox => {
                            if bbox__.is_some() {
                                return Err(serde::de::Error::duplicate_field("bbox"));
                            }
                            bbox__ = map_.next_value()?;
                        }
                        GeneratedField::Mime => {
                            if mime__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mime"));
                            }
                            mime__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Data => {
                            if data__.is_some() {
                                return Err(serde::de::Error::duplicate_field("data"));
                            }
                            data__ = 
                                Some(map_.next_value::<::pbjson::private::BytesDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::RotationDeg => {
                            if rotation_deg__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rotationDeg"));
                            }
                            rotation_deg__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Mirror => {
                            if mirror__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mirror"));
                            }
                            mirror__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Asset => {
                            if asset__.is_some() {
                                return Err(serde::de::Error::duplicate_field("asset"));
                            }
                            asset__ = map_.next_value()?;
                        }
                    }
                }
                Ok(Image {
                    bbox: bbox__,
                    mime: mime__.unwrap_or_default(),
                    data: data__.unwrap_or_default(),
                    rotation_deg: rotation_deg__.unwrap_or_default(),
                    mirror: mirror__.unwrap_or_default(),
                    asset: asset__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Image", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for KeyValue {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.key.is_empty() {
            len += 1;
        }
        if !self.value.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.KeyValue", len)?;
        if !self.key.is_empty() {
            struct_ser.serialize_field("key", &self.key)?;
        }
        if !self.value.is_empty() {
            struct_ser.serialize_field("value", &self.value)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for KeyValue {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "key",
            "value",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Key,
            Value,
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
                            "key" => Ok(GeneratedField::Key),
                            "value" => Ok(GeneratedField::Value),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = KeyValue;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.KeyValue")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<KeyValue, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut key__ = None;
                let mut value__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Key => {
                            if key__.is_some() {
                                return Err(serde::de::Error::duplicate_field("key"));
                            }
                            key__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Value => {
                            if value__.is_some() {
                                return Err(serde::de::Error::duplicate_field("value"));
                            }
                            value__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(KeyValue {
                    key: key__.unwrap_or_default(),
                    value: value__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.KeyValue", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Label {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.text.is_empty() {
            len += 1;
        }
        if self.origin.is_some() {
            len += 1;
        }
        if self.height != 0 {
            len += 1;
        }
        if !self.justify.is_empty() {
            len += 1;
        }
        if self.rotation_deg != 0 {
            len += 1;
        }
        if self.flag != 0 {
            len += 1;
        }
        if self.anchor.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Label", len)?;
        if !self.text.is_empty() {
            struct_ser.serialize_field("text", &self.text)?;
        }
        if let Some(v) = self.origin.as_ref() {
            struct_ser.serialize_field("origin", v)?;
        }
        if self.height != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("height", ToString::to_string(&self.height).as_str())?;
        }
        if !self.justify.is_empty() {
            struct_ser.serialize_field("justify", &self.justify)?;
        }
        if self.rotation_deg != 0 {
            struct_ser.serialize_field("rotationDeg", &self.rotation_deg)?;
        }
        if self.flag != 0 {
            let v = label::Flag::try_from(self.flag)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.flag)))?;
            struct_ser.serialize_field("flag", &v)?;
        }
        if let Some(v) = self.anchor.as_ref() {
            struct_ser.serialize_field("anchor", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Label {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "text",
            "origin",
            "height",
            "justify",
            "rotation_deg",
            "rotationDeg",
            "flag",
            "anchor",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Text,
            Origin,
            Height,
            Justify,
            RotationDeg,
            Flag,
            Anchor,
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
                            "text" => Ok(GeneratedField::Text),
                            "origin" => Ok(GeneratedField::Origin),
                            "height" => Ok(GeneratedField::Height),
                            "justify" => Ok(GeneratedField::Justify),
                            "rotationDeg" | "rotation_deg" => Ok(GeneratedField::RotationDeg),
                            "flag" => Ok(GeneratedField::Flag),
                            "anchor" => Ok(GeneratedField::Anchor),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Label;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.Label")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Label, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut text__ = None;
                let mut origin__ = None;
                let mut height__ = None;
                let mut justify__ = None;
                let mut rotation_deg__ = None;
                let mut flag__ = None;
                let mut anchor__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Text => {
                            if text__.is_some() {
                                return Err(serde::de::Error::duplicate_field("text"));
                            }
                            text__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Origin => {
                            if origin__.is_some() {
                                return Err(serde::de::Error::duplicate_field("origin"));
                            }
                            origin__ = map_.next_value()?;
                        }
                        GeneratedField::Height => {
                            if height__.is_some() {
                                return Err(serde::de::Error::duplicate_field("height"));
                            }
                            height__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Justify => {
                            if justify__.is_some() {
                                return Err(serde::de::Error::duplicate_field("justify"));
                            }
                            justify__ = Some(map_.next_value()?);
                        }
                        GeneratedField::RotationDeg => {
                            if rotation_deg__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rotationDeg"));
                            }
                            rotation_deg__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Flag => {
                            if flag__.is_some() {
                                return Err(serde::de::Error::duplicate_field("flag"));
                            }
                            flag__ = Some(map_.next_value::<label::Flag>()? as i32);
                        }
                        GeneratedField::Anchor => {
                            if anchor__.is_some() {
                                return Err(serde::de::Error::duplicate_field("anchor"));
                            }
                            anchor__ = map_.next_value()?;
                        }
                    }
                }
                Ok(Label {
                    text: text__.unwrap_or_default(),
                    origin: origin__,
                    height: height__.unwrap_or_default(),
                    justify: justify__.unwrap_or_default(),
                    rotation_deg: rotation_deg__.unwrap_or_default(),
                    flag: flag__.unwrap_or_default(),
                    anchor: anchor__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Label", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for label::Flag {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::None => "FLAG_NONE",
            Self::Passive => "FLAG_PASSIVE",
            Self::Input => "FLAG_INPUT",
            Self::Output => "FLAG_OUTPUT",
            Self::Bidirectional => "FLAG_BIDIRECTIONAL",
            Self::TriState => "FLAG_TRI_STATE",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for label::Flag {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "FLAG_NONE",
            "FLAG_PASSIVE",
            "FLAG_INPUT",
            "FLAG_OUTPUT",
            "FLAG_BIDIRECTIONAL",
            "FLAG_TRI_STATE",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = label::Flag;

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
                    "FLAG_NONE" => Ok(label::Flag::None),
                    "FLAG_PASSIVE" => Ok(label::Flag::Passive),
                    "FLAG_INPUT" => Ok(label::Flag::Input),
                    "FLAG_OUTPUT" => Ok(label::Flag::Output),
                    "FLAG_BIDIRECTIONAL" => Ok(label::Flag::Bidirectional),
                    "FLAG_TRI_STATE" => Ok(label::Flag::TriState),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for NetCopper {
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
        if !self.segments.is_empty() {
            len += 1;
        }
        if !self.vias.is_empty() {
            len += 1;
        }
        if !self.shapes.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.NetCopper", len)?;
        if !self.net.is_empty() {
            struct_ser.serialize_field("net", &self.net)?;
        }
        if !self.segments.is_empty() {
            struct_ser.serialize_field("segments", &self.segments)?;
        }
        if !self.vias.is_empty() {
            struct_ser.serialize_field("vias", &self.vias)?;
        }
        if !self.shapes.is_empty() {
            struct_ser.serialize_field("shapes", &self.shapes)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for NetCopper {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "net",
            "segments",
            "vias",
            "shapes",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Net,
            Segments,
            Vias,
            Shapes,
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
                            "segments" => Ok(GeneratedField::Segments),
                            "vias" => Ok(GeneratedField::Vias),
                            "shapes" => Ok(GeneratedField::Shapes),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = NetCopper;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.NetCopper")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<NetCopper, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut net__ = None;
                let mut segments__ = None;
                let mut vias__ = None;
                let mut shapes__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Net => {
                            if net__.is_some() {
                                return Err(serde::de::Error::duplicate_field("net"));
                            }
                            net__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Segments => {
                            if segments__.is_some() {
                                return Err(serde::de::Error::duplicate_field("segments"));
                            }
                            segments__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Vias => {
                            if vias__.is_some() {
                                return Err(serde::de::Error::duplicate_field("vias"));
                            }
                            vias__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Shapes => {
                            if shapes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("shapes"));
                            }
                            shapes__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(NetCopper {
                    net: net__.unwrap_or_default(),
                    segments: segments__.unwrap_or_default(),
                    vias: vias__.unwrap_or_default(),
                    shapes: shapes__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.NetCopper", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PackedHighlight {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.groups.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.PackedHighlight", len)?;
        if !self.groups.is_empty() {
            struct_ser.serialize_field("groups", &self.groups)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PackedHighlight {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "groups",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Groups,
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
                            "groups" => Ok(GeneratedField::Groups),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PackedHighlight;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.PackedHighlight")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PackedHighlight, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut groups__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Groups => {
                            if groups__.is_some() {
                                return Err(serde::de::Error::duplicate_field("groups"));
                            }
                            groups__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(PackedHighlight {
                    groups: groups__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.PackedHighlight", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for packed_highlight::Group {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.color.is_empty() {
            len += 1;
        }
        if self.alpha != 0. {
            len += 1;
        }
        if !self.primitives.is_empty() {
            len += 1;
        }
        if self.shape != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.PackedHighlight.Group", len)?;
        if !self.color.is_empty() {
            struct_ser.serialize_field("color", &self.color)?;
        }
        if self.alpha != 0. {
            struct_ser.serialize_field("alpha", &self.alpha)?;
        }
        if !self.primitives.is_empty() {
            struct_ser.serialize_field("primitives", &self.primitives)?;
        }
        if self.shape != 0 {
            let v = HighlightShape::try_from(self.shape)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.shape)))?;
            struct_ser.serialize_field("shape", &v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for packed_highlight::Group {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "color",
            "alpha",
            "primitives",
            "shape",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Color,
            Alpha,
            Primitives,
            Shape,
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
                            "color" => Ok(GeneratedField::Color),
                            "alpha" => Ok(GeneratedField::Alpha),
                            "primitives" => Ok(GeneratedField::Primitives),
                            "shape" => Ok(GeneratedField::Shape),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = packed_highlight::Group;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.PackedHighlight.Group")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<packed_highlight::Group, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut color__ = None;
                let mut alpha__ = None;
                let mut primitives__ = None;
                let mut shape__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Color => {
                            if color__.is_some() {
                                return Err(serde::de::Error::duplicate_field("color"));
                            }
                            color__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Alpha => {
                            if alpha__.is_some() {
                                return Err(serde::de::Error::duplicate_field("alpha"));
                            }
                            alpha__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Primitives => {
                            if primitives__.is_some() {
                                return Err(serde::de::Error::duplicate_field("primitives"));
                            }
                            primitives__ = 
                                Some(map_.next_value::<Vec<::pbjson::private::NumberDeserialize<_>>>()?
                                    .into_iter().map(|x| x.0).collect())
                            ;
                        }
                        GeneratedField::Shape => {
                            if shape__.is_some() {
                                return Err(serde::de::Error::duplicate_field("shape"));
                            }
                            shape__ = Some(map_.next_value::<HighlightShape>()? as i32);
                        }
                    }
                }
                Ok(packed_highlight::Group {
                    color: color__.unwrap_or_default(),
                    alpha: alpha__.unwrap_or_default(),
                    primitives: primitives__.unwrap_or_default(),
                    shape: shape__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.PackedHighlight.Group", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PackedImage {
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
        if self.w != 0 {
            len += 1;
        }
        if self.h != 0 {
            len += 1;
        }
        if !self.mime.is_empty() {
            len += 1;
        }
        if !self.data.is_empty() {
            len += 1;
        }
        if self.rotation_deg != 0 {
            len += 1;
        }
        if self.mirror {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.PackedImage", len)?;
        if self.x != 0 {
            struct_ser.serialize_field("x", &self.x)?;
        }
        if self.y != 0 {
            struct_ser.serialize_field("y", &self.y)?;
        }
        if self.w != 0 {
            struct_ser.serialize_field("w", &self.w)?;
        }
        if self.h != 0 {
            struct_ser.serialize_field("h", &self.h)?;
        }
        if !self.mime.is_empty() {
            struct_ser.serialize_field("mime", &self.mime)?;
        }
        if !self.data.is_empty() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("data", pbjson::private::base64::encode(&self.data).as_str())?;
        }
        if self.rotation_deg != 0 {
            struct_ser.serialize_field("rotationDeg", &self.rotation_deg)?;
        }
        if self.mirror {
            struct_ser.serialize_field("mirror", &self.mirror)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PackedImage {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "x",
            "y",
            "w",
            "h",
            "mime",
            "data",
            "rotation_deg",
            "rotationDeg",
            "mirror",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            X,
            Y,
            W,
            H,
            Mime,
            Data,
            RotationDeg,
            Mirror,
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
                            "w" => Ok(GeneratedField::W),
                            "h" => Ok(GeneratedField::H),
                            "mime" => Ok(GeneratedField::Mime),
                            "data" => Ok(GeneratedField::Data),
                            "rotationDeg" | "rotation_deg" => Ok(GeneratedField::RotationDeg),
                            "mirror" => Ok(GeneratedField::Mirror),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PackedImage;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.PackedImage")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PackedImage, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut x__ = None;
                let mut y__ = None;
                let mut w__ = None;
                let mut h__ = None;
                let mut mime__ = None;
                let mut data__ = None;
                let mut rotation_deg__ = None;
                let mut mirror__ = None;
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
                        GeneratedField::W => {
                            if w__.is_some() {
                                return Err(serde::de::Error::duplicate_field("w"));
                            }
                            w__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::H => {
                            if h__.is_some() {
                                return Err(serde::de::Error::duplicate_field("h"));
                            }
                            h__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Mime => {
                            if mime__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mime"));
                            }
                            mime__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Data => {
                            if data__.is_some() {
                                return Err(serde::de::Error::duplicate_field("data"));
                            }
                            data__ = 
                                Some(map_.next_value::<::pbjson::private::BytesDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::RotationDeg => {
                            if rotation_deg__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rotationDeg"));
                            }
                            rotation_deg__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Mirror => {
                            if mirror__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mirror"));
                            }
                            mirror__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(PackedImage {
                    x: x__.unwrap_or_default(),
                    y: y__.unwrap_or_default(),
                    w: w__.unwrap_or_default(),
                    h: h__.unwrap_or_default(),
                    mime: mime__.unwrap_or_default(),
                    data: data__.unwrap_or_default(),
                    rotation_deg: rotation_deg__.unwrap_or_default(),
                    mirror: mirror__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.PackedImage", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PackedKeys {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.strings.is_empty() {
            len += 1;
        }
        if !self.records.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.PackedKeys", len)?;
        if !self.strings.is_empty() {
            struct_ser.serialize_field("strings", &self.strings)?;
        }
        if !self.records.is_empty() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("records", pbjson::private::base64::encode(&self.records).as_str())?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PackedKeys {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "strings",
            "records",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Strings,
            Records,
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
                            "strings" => Ok(GeneratedField::Strings),
                            "records" => Ok(GeneratedField::Records),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PackedKeys;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.PackedKeys")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PackedKeys, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut strings__ = None;
                let mut records__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Strings => {
                            if strings__.is_some() {
                                return Err(serde::de::Error::duplicate_field("strings"));
                            }
                            strings__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Records => {
                            if records__.is_some() {
                                return Err(serde::de::Error::duplicate_field("records"));
                            }
                            records__ = 
                                Some(map_.next_value::<::pbjson::private::BytesDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(PackedKeys {
                    strings: strings__.unwrap_or_default(),
                    records: records__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.PackedKeys", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PackedLabel {
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
        if !self.text.is_empty() {
            len += 1;
        }
        if self.height != 0 {
            len += 1;
        }
        if self.rotation_deg != 0 {
            len += 1;
        }
        if !self.justify.is_empty() {
            len += 1;
        }
        if !self.color.is_empty() {
            len += 1;
        }
        if self.max_width != 0 {
            len += 1;
        }
        if !self.runs.is_empty() {
            len += 1;
        }
        if self.group != 0 {
            len += 1;
        }
        if self.mirror {
            len += 1;
        }
        if self.width != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.PackedLabel", len)?;
        if self.x != 0 {
            struct_ser.serialize_field("x", &self.x)?;
        }
        if self.y != 0 {
            struct_ser.serialize_field("y", &self.y)?;
        }
        if !self.text.is_empty() {
            struct_ser.serialize_field("text", &self.text)?;
        }
        if self.height != 0 {
            struct_ser.serialize_field("height", &self.height)?;
        }
        if self.rotation_deg != 0 {
            struct_ser.serialize_field("rotationDeg", &self.rotation_deg)?;
        }
        if !self.justify.is_empty() {
            struct_ser.serialize_field("justify", &self.justify)?;
        }
        if !self.color.is_empty() {
            struct_ser.serialize_field("color", &self.color)?;
        }
        if self.max_width != 0 {
            struct_ser.serialize_field("maxWidth", &self.max_width)?;
        }
        if !self.runs.is_empty() {
            struct_ser.serialize_field("runs", &self.runs)?;
        }
        if self.group != 0 {
            struct_ser.serialize_field("group", &self.group)?;
        }
        if self.mirror {
            struct_ser.serialize_field("mirror", &self.mirror)?;
        }
        if self.width != 0 {
            struct_ser.serialize_field("width", &self.width)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PackedLabel {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "x",
            "y",
            "text",
            "height",
            "rotation_deg",
            "rotationDeg",
            "justify",
            "color",
            "max_width",
            "maxWidth",
            "runs",
            "group",
            "mirror",
            "width",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            X,
            Y,
            Text,
            Height,
            RotationDeg,
            Justify,
            Color,
            MaxWidth,
            Runs,
            Group,
            Mirror,
            Width,
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
                            "text" => Ok(GeneratedField::Text),
                            "height" => Ok(GeneratedField::Height),
                            "rotationDeg" | "rotation_deg" => Ok(GeneratedField::RotationDeg),
                            "justify" => Ok(GeneratedField::Justify),
                            "color" => Ok(GeneratedField::Color),
                            "maxWidth" | "max_width" => Ok(GeneratedField::MaxWidth),
                            "runs" => Ok(GeneratedField::Runs),
                            "group" => Ok(GeneratedField::Group),
                            "mirror" => Ok(GeneratedField::Mirror),
                            "width" => Ok(GeneratedField::Width),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PackedLabel;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.PackedLabel")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PackedLabel, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut x__ = None;
                let mut y__ = None;
                let mut text__ = None;
                let mut height__ = None;
                let mut rotation_deg__ = None;
                let mut justify__ = None;
                let mut color__ = None;
                let mut max_width__ = None;
                let mut runs__ = None;
                let mut group__ = None;
                let mut mirror__ = None;
                let mut width__ = None;
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
                        GeneratedField::Text => {
                            if text__.is_some() {
                                return Err(serde::de::Error::duplicate_field("text"));
                            }
                            text__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Height => {
                            if height__.is_some() {
                                return Err(serde::de::Error::duplicate_field("height"));
                            }
                            height__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::RotationDeg => {
                            if rotation_deg__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rotationDeg"));
                            }
                            rotation_deg__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Justify => {
                            if justify__.is_some() {
                                return Err(serde::de::Error::duplicate_field("justify"));
                            }
                            justify__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Color => {
                            if color__.is_some() {
                                return Err(serde::de::Error::duplicate_field("color"));
                            }
                            color__ = Some(map_.next_value()?);
                        }
                        GeneratedField::MaxWidth => {
                            if max_width__.is_some() {
                                return Err(serde::de::Error::duplicate_field("maxWidth"));
                            }
                            max_width__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Runs => {
                            if runs__.is_some() {
                                return Err(serde::de::Error::duplicate_field("runs"));
                            }
                            runs__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Group => {
                            if group__.is_some() {
                                return Err(serde::de::Error::duplicate_field("group"));
                            }
                            group__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Mirror => {
                            if mirror__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mirror"));
                            }
                            mirror__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Width => {
                            if width__.is_some() {
                                return Err(serde::de::Error::duplicate_field("width"));
                            }
                            width__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(PackedLabel {
                    x: x__.unwrap_or_default(),
                    y: y__.unwrap_or_default(),
                    text: text__.unwrap_or_default(),
                    height: height__.unwrap_or_default(),
                    rotation_deg: rotation_deg__.unwrap_or_default(),
                    justify: justify__.unwrap_or_default(),
                    color: color__.unwrap_or_default(),
                    max_width: max_width__.unwrap_or_default(),
                    runs: runs__.unwrap_or_default(),
                    group: group__.unwrap_or_default(),
                    mirror: mirror__.unwrap_or_default(),
                    width: width__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.PackedLabel", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PackedLabelRuns {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.label != 0 {
            len += 1;
        }
        if !self.runs.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.PackedLabelRuns", len)?;
        if self.label != 0 {
            struct_ser.serialize_field("label", &self.label)?;
        }
        if !self.runs.is_empty() {
            struct_ser.serialize_field("runs", &self.runs)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PackedLabelRuns {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "label",
            "runs",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Label,
            Runs,
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
                            "runs" => Ok(GeneratedField::Runs),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PackedLabelRuns;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.PackedLabelRuns")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PackedLabelRuns, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut label__ = None;
                let mut runs__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Label => {
                            if label__.is_some() {
                                return Err(serde::de::Error::duplicate_field("label"));
                            }
                            label__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Runs => {
                            if runs__.is_some() {
                                return Err(serde::de::Error::duplicate_field("runs"));
                            }
                            runs__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(PackedLabelRuns {
                    label: label__.unwrap_or_default(),
                    runs: runs__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.PackedLabelRuns", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PackedLabels {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.strings.is_empty() {
            len += 1;
        }
        if !self.records.is_empty() {
            len += 1;
        }
        if !self.runs.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.PackedLabels", len)?;
        if !self.strings.is_empty() {
            struct_ser.serialize_field("strings", &self.strings)?;
        }
        if !self.records.is_empty() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("records", pbjson::private::base64::encode(&self.records).as_str())?;
        }
        if !self.runs.is_empty() {
            struct_ser.serialize_field("runs", &self.runs)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PackedLabels {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "strings",
            "records",
            "runs",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Strings,
            Records,
            Runs,
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
                            "strings" => Ok(GeneratedField::Strings),
                            "records" => Ok(GeneratedField::Records),
                            "runs" => Ok(GeneratedField::Runs),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PackedLabels;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.PackedLabels")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PackedLabels, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut strings__ = None;
                let mut records__ = None;
                let mut runs__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Strings => {
                            if strings__.is_some() {
                                return Err(serde::de::Error::duplicate_field("strings"));
                            }
                            strings__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Records => {
                            if records__.is_some() {
                                return Err(serde::de::Error::duplicate_field("records"));
                            }
                            records__ = 
                                Some(map_.next_value::<::pbjson::private::BytesDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Runs => {
                            if runs__.is_some() {
                                return Err(serde::de::Error::duplicate_field("runs"));
                            }
                            runs__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(PackedLabels {
                    strings: strings__.unwrap_or_default(),
                    records: records__.unwrap_or_default(),
                    runs: runs__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.PackedLabels", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PackedSheet {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.sheet_id.is_empty() {
            len += 1;
        }
        if self.layout_version != 0 {
            len += 1;
        }
        if self.origin_x != 0 {
            len += 1;
        }
        if self.origin_y != 0 {
            len += 1;
        }
        if !self.vertices.is_empty() {
            len += 1;
        }
        if !self.primitives.is_empty() {
            len += 1;
        }
        if !self.font_family.is_empty() {
            len += 1;
        }
        if !self.group_colors.is_empty() {
            len += 1;
        }
        if !self.background_color.is_empty() {
            len += 1;
        }
        if !self.images.is_empty() {
            len += 1;
        }
        if !self.group_alphas.is_empty() {
            len += 1;
        }
        if self.key_columns.is_some() {
            len += 1;
        }
        if self.label_columns.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.PackedSheet", len)?;
        if !self.sheet_id.is_empty() {
            struct_ser.serialize_field("sheetId", &self.sheet_id)?;
        }
        if self.layout_version != 0 {
            struct_ser.serialize_field("layoutVersion", &self.layout_version)?;
        }
        if self.origin_x != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("originX", ToString::to_string(&self.origin_x).as_str())?;
        }
        if self.origin_y != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("originY", ToString::to_string(&self.origin_y).as_str())?;
        }
        if !self.vertices.is_empty() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("vertices", pbjson::private::base64::encode(&self.vertices).as_str())?;
        }
        if !self.primitives.is_empty() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("primitives", pbjson::private::base64::encode(&self.primitives).as_str())?;
        }
        if !self.font_family.is_empty() {
            struct_ser.serialize_field("fontFamily", &self.font_family)?;
        }
        if !self.group_colors.is_empty() {
            struct_ser.serialize_field("groupColors", &self.group_colors)?;
        }
        if !self.background_color.is_empty() {
            struct_ser.serialize_field("backgroundColor", &self.background_color)?;
        }
        if !self.images.is_empty() {
            struct_ser.serialize_field("images", &self.images)?;
        }
        if !self.group_alphas.is_empty() {
            struct_ser.serialize_field("groupAlphas", &self.group_alphas)?;
        }
        if let Some(v) = self.key_columns.as_ref() {
            struct_ser.serialize_field("keyColumns", v)?;
        }
        if let Some(v) = self.label_columns.as_ref() {
            struct_ser.serialize_field("labelColumns", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PackedSheet {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "sheet_id",
            "sheetId",
            "layout_version",
            "layoutVersion",
            "origin_x",
            "originX",
            "origin_y",
            "originY",
            "vertices",
            "primitives",
            "font_family",
            "fontFamily",
            "group_colors",
            "groupColors",
            "background_color",
            "backgroundColor",
            "images",
            "group_alphas",
            "groupAlphas",
            "key_columns",
            "keyColumns",
            "label_columns",
            "labelColumns",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            SheetId,
            LayoutVersion,
            OriginX,
            OriginY,
            Vertices,
            Primitives,
            FontFamily,
            GroupColors,
            BackgroundColor,
            Images,
            GroupAlphas,
            KeyColumns,
            LabelColumns,
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
                            "sheetId" | "sheet_id" => Ok(GeneratedField::SheetId),
                            "layoutVersion" | "layout_version" => Ok(GeneratedField::LayoutVersion),
                            "originX" | "origin_x" => Ok(GeneratedField::OriginX),
                            "originY" | "origin_y" => Ok(GeneratedField::OriginY),
                            "vertices" => Ok(GeneratedField::Vertices),
                            "primitives" => Ok(GeneratedField::Primitives),
                            "fontFamily" | "font_family" => Ok(GeneratedField::FontFamily),
                            "groupColors" | "group_colors" => Ok(GeneratedField::GroupColors),
                            "backgroundColor" | "background_color" => Ok(GeneratedField::BackgroundColor),
                            "images" => Ok(GeneratedField::Images),
                            "groupAlphas" | "group_alphas" => Ok(GeneratedField::GroupAlphas),
                            "keyColumns" | "key_columns" => Ok(GeneratedField::KeyColumns),
                            "labelColumns" | "label_columns" => Ok(GeneratedField::LabelColumns),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PackedSheet;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.PackedSheet")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PackedSheet, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut sheet_id__ = None;
                let mut layout_version__ = None;
                let mut origin_x__ = None;
                let mut origin_y__ = None;
                let mut vertices__ = None;
                let mut primitives__ = None;
                let mut font_family__ = None;
                let mut group_colors__ = None;
                let mut background_color__ = None;
                let mut images__ = None;
                let mut group_alphas__ = None;
                let mut key_columns__ = None;
                let mut label_columns__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::SheetId => {
                            if sheet_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sheetId"));
                            }
                            sheet_id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::LayoutVersion => {
                            if layout_version__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layoutVersion"));
                            }
                            layout_version__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::OriginX => {
                            if origin_x__.is_some() {
                                return Err(serde::de::Error::duplicate_field("originX"));
                            }
                            origin_x__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::OriginY => {
                            if origin_y__.is_some() {
                                return Err(serde::de::Error::duplicate_field("originY"));
                            }
                            origin_y__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Vertices => {
                            if vertices__.is_some() {
                                return Err(serde::de::Error::duplicate_field("vertices"));
                            }
                            vertices__ = 
                                Some(map_.next_value::<::pbjson::private::BytesDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Primitives => {
                            if primitives__.is_some() {
                                return Err(serde::de::Error::duplicate_field("primitives"));
                            }
                            primitives__ = 
                                Some(map_.next_value::<::pbjson::private::BytesDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::FontFamily => {
                            if font_family__.is_some() {
                                return Err(serde::de::Error::duplicate_field("fontFamily"));
                            }
                            font_family__ = Some(map_.next_value()?);
                        }
                        GeneratedField::GroupColors => {
                            if group_colors__.is_some() {
                                return Err(serde::de::Error::duplicate_field("groupColors"));
                            }
                            group_colors__ = Some(map_.next_value()?);
                        }
                        GeneratedField::BackgroundColor => {
                            if background_color__.is_some() {
                                return Err(serde::de::Error::duplicate_field("backgroundColor"));
                            }
                            background_color__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Images => {
                            if images__.is_some() {
                                return Err(serde::de::Error::duplicate_field("images"));
                            }
                            images__ = Some(map_.next_value()?);
                        }
                        GeneratedField::GroupAlphas => {
                            if group_alphas__.is_some() {
                                return Err(serde::de::Error::duplicate_field("groupAlphas"));
                            }
                            group_alphas__ = 
                                Some(map_.next_value::<Vec<::pbjson::private::NumberDeserialize<_>>>()?
                                    .into_iter().map(|x| x.0).collect())
                            ;
                        }
                        GeneratedField::KeyColumns => {
                            if key_columns__.is_some() {
                                return Err(serde::de::Error::duplicate_field("keyColumns"));
                            }
                            key_columns__ = map_.next_value()?;
                        }
                        GeneratedField::LabelColumns => {
                            if label_columns__.is_some() {
                                return Err(serde::de::Error::duplicate_field("labelColumns"));
                            }
                            label_columns__ = map_.next_value()?;
                        }
                    }
                }
                Ok(PackedSheet {
                    sheet_id: sheet_id__.unwrap_or_default(),
                    layout_version: layout_version__.unwrap_or_default(),
                    origin_x: origin_x__.unwrap_or_default(),
                    origin_y: origin_y__.unwrap_or_default(),
                    vertices: vertices__.unwrap_or_default(),
                    primitives: primitives__.unwrap_or_default(),
                    font_family: font_family__.unwrap_or_default(),
                    group_colors: group_colors__.unwrap_or_default(),
                    background_color: background_color__.unwrap_or_default(),
                    images: images__.unwrap_or_default(),
                    group_alphas: group_alphas__.unwrap_or_default(),
                    key_columns: key_columns__,
                    label_columns: label_columns__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.PackedSheet", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Pad {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.number.is_empty() {
            len += 1;
        }
        if self.at.is_some() {
            len += 1;
        }
        if self.rotation_deg != 0. {
            len += 1;
        }
        if self.size.is_some() {
            len += 1;
        }
        if !self.shape.is_empty() {
            len += 1;
        }
        if !self.layers.is_empty() {
            len += 1;
        }
        if self.drill != 0 {
            len += 1;
        }
        if !self.net.is_empty() {
            len += 1;
        }
        if !self.padstack.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Pad", len)?;
        if !self.number.is_empty() {
            struct_ser.serialize_field("number", &self.number)?;
        }
        if let Some(v) = self.at.as_ref() {
            struct_ser.serialize_field("at", v)?;
        }
        if self.rotation_deg != 0. {
            struct_ser.serialize_field("rotationDeg", &self.rotation_deg)?;
        }
        if let Some(v) = self.size.as_ref() {
            struct_ser.serialize_field("size", v)?;
        }
        if !self.shape.is_empty() {
            struct_ser.serialize_field("shape", &self.shape)?;
        }
        if !self.layers.is_empty() {
            struct_ser.serialize_field("layers", &self.layers)?;
        }
        if self.drill != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("drill", ToString::to_string(&self.drill).as_str())?;
        }
        if !self.net.is_empty() {
            struct_ser.serialize_field("net", &self.net)?;
        }
        if !self.padstack.is_empty() {
            struct_ser.serialize_field("padstack", &self.padstack)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Pad {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "number",
            "at",
            "rotation_deg",
            "rotationDeg",
            "size",
            "shape",
            "layers",
            "drill",
            "net",
            "padstack",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Number,
            At,
            RotationDeg,
            Size,
            Shape,
            Layers,
            Drill,
            Net,
            Padstack,
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
                            "number" => Ok(GeneratedField::Number),
                            "at" => Ok(GeneratedField::At),
                            "rotationDeg" | "rotation_deg" => Ok(GeneratedField::RotationDeg),
                            "size" => Ok(GeneratedField::Size),
                            "shape" => Ok(GeneratedField::Shape),
                            "layers" => Ok(GeneratedField::Layers),
                            "drill" => Ok(GeneratedField::Drill),
                            "net" => Ok(GeneratedField::Net),
                            "padstack" => Ok(GeneratedField::Padstack),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Pad;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.Pad")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Pad, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut number__ = None;
                let mut at__ = None;
                let mut rotation_deg__ = None;
                let mut size__ = None;
                let mut shape__ = None;
                let mut layers__ = None;
                let mut drill__ = None;
                let mut net__ = None;
                let mut padstack__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Number => {
                            if number__.is_some() {
                                return Err(serde::de::Error::duplicate_field("number"));
                            }
                            number__ = Some(map_.next_value()?);
                        }
                        GeneratedField::At => {
                            if at__.is_some() {
                                return Err(serde::de::Error::duplicate_field("at"));
                            }
                            at__ = map_.next_value()?;
                        }
                        GeneratedField::RotationDeg => {
                            if rotation_deg__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rotationDeg"));
                            }
                            rotation_deg__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Size => {
                            if size__.is_some() {
                                return Err(serde::de::Error::duplicate_field("size"));
                            }
                            size__ = map_.next_value()?;
                        }
                        GeneratedField::Shape => {
                            if shape__.is_some() {
                                return Err(serde::de::Error::duplicate_field("shape"));
                            }
                            shape__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Layers => {
                            if layers__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layers"));
                            }
                            layers__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Drill => {
                            if drill__.is_some() {
                                return Err(serde::de::Error::duplicate_field("drill"));
                            }
                            drill__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Net => {
                            if net__.is_some() {
                                return Err(serde::de::Error::duplicate_field("net"));
                            }
                            net__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Padstack => {
                            if padstack__.is_some() {
                                return Err(serde::de::Error::duplicate_field("padstack"));
                            }
                            padstack__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Pad {
                    number: number__.unwrap_or_default(),
                    at: at__,
                    rotation_deg: rotation_deg__.unwrap_or_default(),
                    size: size__,
                    shape: shape__.unwrap_or_default(),
                    layers: layers__.unwrap_or_default(),
                    drill: drill__.unwrap_or_default(),
                    net: net__.unwrap_or_default(),
                    padstack: padstack__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Pad", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PadShape {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.shape.is_empty() {
            len += 1;
        }
        if self.size.is_some() {
            len += 1;
        }
        if self.offset.is_some() {
            len += 1;
        }
        if !self.outline.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.PadShape", len)?;
        if !self.shape.is_empty() {
            struct_ser.serialize_field("shape", &self.shape)?;
        }
        if let Some(v) = self.size.as_ref() {
            struct_ser.serialize_field("size", v)?;
        }
        if let Some(v) = self.offset.as_ref() {
            struct_ser.serialize_field("offset", v)?;
        }
        if !self.outline.is_empty() {
            struct_ser.serialize_field("outline", &self.outline)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PadShape {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "shape",
            "size",
            "offset",
            "outline",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Shape,
            Size,
            Offset,
            Outline,
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
                            "shape" => Ok(GeneratedField::Shape),
                            "size" => Ok(GeneratedField::Size),
                            "offset" => Ok(GeneratedField::Offset),
                            "outline" => Ok(GeneratedField::Outline),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PadShape;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.PadShape")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PadShape, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut shape__ = None;
                let mut size__ = None;
                let mut offset__ = None;
                let mut outline__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Shape => {
                            if shape__.is_some() {
                                return Err(serde::de::Error::duplicate_field("shape"));
                            }
                            shape__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Size => {
                            if size__.is_some() {
                                return Err(serde::de::Error::duplicate_field("size"));
                            }
                            size__ = map_.next_value()?;
                        }
                        GeneratedField::Offset => {
                            if offset__.is_some() {
                                return Err(serde::de::Error::duplicate_field("offset"));
                            }
                            offset__ = map_.next_value()?;
                        }
                        GeneratedField::Outline => {
                            if outline__.is_some() {
                                return Err(serde::de::Error::duplicate_field("outline"));
                            }
                            outline__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(PadShape {
                    shape: shape__.unwrap_or_default(),
                    size: size__,
                    offset: offset__,
                    outline: outline__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.PadShape", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Padstack {
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
        if !self.layers.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Padstack", len)?;
        if !self.id.is_empty() {
            struct_ser.serialize_field("id", &self.id)?;
        }
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.layers.is_empty() {
            struct_ser.serialize_field("layers", &self.layers)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Padstack {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "id",
            "name",
            "layers",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Id,
            Name,
            Layers,
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
                            "layers" => Ok(GeneratedField::Layers),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Padstack;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.Padstack")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Padstack, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut id__ = None;
                let mut name__ = None;
                let mut layers__ = None;
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
                        GeneratedField::Layers => {
                            if layers__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layers"));
                            }
                            layers__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Padstack {
                    id: id__.unwrap_or_default(),
                    name: name__.unwrap_or_default(),
                    layers: layers__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Padstack", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PadstackLayer {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.layer.is_empty() {
            len += 1;
        }
        if self.pad.is_some() {
            len += 1;
        }
        if self.thermal.is_some() {
            len += 1;
        }
        if self.antipad.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.PadstackLayer", len)?;
        if !self.layer.is_empty() {
            struct_ser.serialize_field("layer", &self.layer)?;
        }
        if let Some(v) = self.pad.as_ref() {
            struct_ser.serialize_field("pad", v)?;
        }
        if let Some(v) = self.thermal.as_ref() {
            struct_ser.serialize_field("thermal", v)?;
        }
        if let Some(v) = self.antipad.as_ref() {
            struct_ser.serialize_field("antipad", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PadstackLayer {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "layer",
            "pad",
            "thermal",
            "antipad",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Layer,
            Pad,
            Thermal,
            Antipad,
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
                            "layer" => Ok(GeneratedField::Layer),
                            "pad" => Ok(GeneratedField::Pad),
                            "thermal" => Ok(GeneratedField::Thermal),
                            "antipad" => Ok(GeneratedField::Antipad),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PadstackLayer;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.PadstackLayer")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PadstackLayer, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut layer__ = None;
                let mut pad__ = None;
                let mut thermal__ = None;
                let mut antipad__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Layer => {
                            if layer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layer"));
                            }
                            layer__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Pad => {
                            if pad__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pad"));
                            }
                            pad__ = map_.next_value()?;
                        }
                        GeneratedField::Thermal => {
                            if thermal__.is_some() {
                                return Err(serde::de::Error::duplicate_field("thermal"));
                            }
                            thermal__ = map_.next_value()?;
                        }
                        GeneratedField::Antipad => {
                            if antipad__.is_some() {
                                return Err(serde::de::Error::duplicate_field("antipad"));
                            }
                            antipad__ = map_.next_value()?;
                        }
                    }
                }
                Ok(PadstackLayer {
                    layer: layer__.unwrap_or_default(),
                    pad: pad__,
                    thermal: thermal__,
                    antipad: antipad__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.PadstackLayer", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PinPoint {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.port_ref.is_empty() {
            len += 1;
        }
        if self.loc.is_some() {
            len += 1;
        }
        if !self.source_id.is_empty() {
            len += 1;
        }
        if self.label_origin.is_some() {
            len += 1;
        }
        if !self.justify.is_empty() {
            len += 1;
        }
        if !self.name.is_empty() {
            len += 1;
        }
        if self.height != 0 {
            len += 1;
        }
        if self.number_origin.is_some() {
            len += 1;
        }
        if !self.number_justify.is_empty() {
            len += 1;
        }
        if self.text_rotation_deg != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.PinPoint", len)?;
        if !self.port_ref.is_empty() {
            struct_ser.serialize_field("portRef", &self.port_ref)?;
        }
        if let Some(v) = self.loc.as_ref() {
            struct_ser.serialize_field("loc", v)?;
        }
        if !self.source_id.is_empty() {
            struct_ser.serialize_field("sourceId", &self.source_id)?;
        }
        if let Some(v) = self.label_origin.as_ref() {
            struct_ser.serialize_field("labelOrigin", v)?;
        }
        if !self.justify.is_empty() {
            struct_ser.serialize_field("justify", &self.justify)?;
        }
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if self.height != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("height", ToString::to_string(&self.height).as_str())?;
        }
        if let Some(v) = self.number_origin.as_ref() {
            struct_ser.serialize_field("numberOrigin", v)?;
        }
        if !self.number_justify.is_empty() {
            struct_ser.serialize_field("numberJustify", &self.number_justify)?;
        }
        if self.text_rotation_deg != 0 {
            struct_ser.serialize_field("textRotationDeg", &self.text_rotation_deg)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PinPoint {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "port_ref",
            "portRef",
            "loc",
            "source_id",
            "sourceId",
            "label_origin",
            "labelOrigin",
            "justify",
            "name",
            "height",
            "number_origin",
            "numberOrigin",
            "number_justify",
            "numberJustify",
            "text_rotation_deg",
            "textRotationDeg",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            PortRef,
            Loc,
            SourceId,
            LabelOrigin,
            Justify,
            Name,
            Height,
            NumberOrigin,
            NumberJustify,
            TextRotationDeg,
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
                            "portRef" | "port_ref" => Ok(GeneratedField::PortRef),
                            "loc" => Ok(GeneratedField::Loc),
                            "sourceId" | "source_id" => Ok(GeneratedField::SourceId),
                            "labelOrigin" | "label_origin" => Ok(GeneratedField::LabelOrigin),
                            "justify" => Ok(GeneratedField::Justify),
                            "name" => Ok(GeneratedField::Name),
                            "height" => Ok(GeneratedField::Height),
                            "numberOrigin" | "number_origin" => Ok(GeneratedField::NumberOrigin),
                            "numberJustify" | "number_justify" => Ok(GeneratedField::NumberJustify),
                            "textRotationDeg" | "text_rotation_deg" => Ok(GeneratedField::TextRotationDeg),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PinPoint;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.PinPoint")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PinPoint, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut port_ref__ = None;
                let mut loc__ = None;
                let mut source_id__ = None;
                let mut label_origin__ = None;
                let mut justify__ = None;
                let mut name__ = None;
                let mut height__ = None;
                let mut number_origin__ = None;
                let mut number_justify__ = None;
                let mut text_rotation_deg__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::PortRef => {
                            if port_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("portRef"));
                            }
                            port_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Loc => {
                            if loc__.is_some() {
                                return Err(serde::de::Error::duplicate_field("loc"));
                            }
                            loc__ = map_.next_value()?;
                        }
                        GeneratedField::SourceId => {
                            if source_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sourceId"));
                            }
                            source_id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::LabelOrigin => {
                            if label_origin__.is_some() {
                                return Err(serde::de::Error::duplicate_field("labelOrigin"));
                            }
                            label_origin__ = map_.next_value()?;
                        }
                        GeneratedField::Justify => {
                            if justify__.is_some() {
                                return Err(serde::de::Error::duplicate_field("justify"));
                            }
                            justify__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Height => {
                            if height__.is_some() {
                                return Err(serde::de::Error::duplicate_field("height"));
                            }
                            height__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::NumberOrigin => {
                            if number_origin__.is_some() {
                                return Err(serde::de::Error::duplicate_field("numberOrigin"));
                            }
                            number_origin__ = map_.next_value()?;
                        }
                        GeneratedField::NumberJustify => {
                            if number_justify__.is_some() {
                                return Err(serde::de::Error::duplicate_field("numberJustify"));
                            }
                            number_justify__ = Some(map_.next_value()?);
                        }
                        GeneratedField::TextRotationDeg => {
                            if text_rotation_deg__.is_some() {
                                return Err(serde::de::Error::duplicate_field("textRotationDeg"));
                            }
                            text_rotation_deg__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(PinPoint {
                    port_ref: port_ref__.unwrap_or_default(),
                    loc: loc__,
                    source_id: source_id__.unwrap_or_default(),
                    label_origin: label_origin__,
                    justify: justify__.unwrap_or_default(),
                    name: name__.unwrap_or_default(),
                    height: height__.unwrap_or_default(),
                    number_origin: number_origin__,
                    number_justify: number_justify__.unwrap_or_default(),
                    text_rotation_deg: text_rotation_deg__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.PinPoint", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PinRef {
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
        if !self.pin.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.PinRef", len)?;
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if !self.pin.is_empty() {
            struct_ser.serialize_field("pin", &self.pin)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PinRef {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "ref_des",
            "refDes",
            "pin",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            RefDes,
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
                            "refDes" | "ref_des" => Ok(GeneratedField::RefDes),
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
            type Value = PinRef;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.PinRef")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PinRef, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut ref_des__ = None;
                let mut pin__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Pin => {
                            if pin__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pin"));
                            }
                            pin__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(PinRef {
                    ref_des: ref_des__.unwrap_or_default(),
                    pin: pin__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.PinRef", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Point {
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
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Point", len)?;
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
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Point {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "x",
            "y",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            X,
            Y,
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
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Point;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.Point")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Point, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut x__ = None;
                let mut y__ = None;
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
                    }
                }
                Ok(Point {
                    x: x__.unwrap_or_default(),
                    y: y__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Point", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Polyline {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.points.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Polyline", len)?;
        if !self.points.is_empty() {
            struct_ser.serialize_field("points", &self.points)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Polyline {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "points",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Points,
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
                            "points" => Ok(GeneratedField::Points),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Polyline;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.Polyline")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Polyline, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut points__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Points => {
                            if points__.is_some() {
                                return Err(serde::de::Error::duplicate_field("points"));
                            }
                            points__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Polyline {
                    points: points__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Polyline", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for PrimitiveKey {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.primitive != 0 {
            len += 1;
        }
        if !self.ref_des.is_empty() {
            len += 1;
        }
        if !self.net.is_empty() {
            len += 1;
        }
        if !self.pin.is_empty() {
            len += 1;
        }
        if !self.net_id.is_empty() {
            len += 1;
        }
        if !self.bus_id.is_empty() {
            len += 1;
        }
        if self.placement != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.PrimitiveKey", len)?;
        if self.primitive != 0 {
            struct_ser.serialize_field("primitive", &self.primitive)?;
        }
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if !self.net.is_empty() {
            struct_ser.serialize_field("net", &self.net)?;
        }
        if !self.pin.is_empty() {
            struct_ser.serialize_field("pin", &self.pin)?;
        }
        if !self.net_id.is_empty() {
            struct_ser.serialize_field("netId", &self.net_id)?;
        }
        if !self.bus_id.is_empty() {
            struct_ser.serialize_field("busId", &self.bus_id)?;
        }
        if self.placement != 0 {
            struct_ser.serialize_field("placement", &self.placement)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for PrimitiveKey {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "primitive",
            "ref_des",
            "refDes",
            "net",
            "pin",
            "net_id",
            "netId",
            "bus_id",
            "busId",
            "placement",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Primitive,
            RefDes,
            Net,
            Pin,
            NetId,
            BusId,
            Placement,
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
                            "primitive" => Ok(GeneratedField::Primitive),
                            "refDes" | "ref_des" => Ok(GeneratedField::RefDes),
                            "net" => Ok(GeneratedField::Net),
                            "pin" => Ok(GeneratedField::Pin),
                            "netId" | "net_id" => Ok(GeneratedField::NetId),
                            "busId" | "bus_id" => Ok(GeneratedField::BusId),
                            "placement" => Ok(GeneratedField::Placement),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = PrimitiveKey;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.PrimitiveKey")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<PrimitiveKey, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut primitive__ = None;
                let mut ref_des__ = None;
                let mut net__ = None;
                let mut pin__ = None;
                let mut net_id__ = None;
                let mut bus_id__ = None;
                let mut placement__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Primitive => {
                            if primitive__.is_some() {
                                return Err(serde::de::Error::duplicate_field("primitive"));
                            }
                            primitive__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Net => {
                            if net__.is_some() {
                                return Err(serde::de::Error::duplicate_field("net"));
                            }
                            net__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Pin => {
                            if pin__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pin"));
                            }
                            pin__ = Some(map_.next_value()?);
                        }
                        GeneratedField::NetId => {
                            if net_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("netId"));
                            }
                            net_id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::BusId => {
                            if bus_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("busId"));
                            }
                            bus_id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Placement => {
                            if placement__.is_some() {
                                return Err(serde::de::Error::duplicate_field("placement"));
                            }
                            placement__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(PrimitiveKey {
                    primitive: primitive__.unwrap_or_default(),
                    ref_des: ref_des__.unwrap_or_default(),
                    net: net__.unwrap_or_default(),
                    pin: pin__.unwrap_or_default(),
                    net_id: net_id__.unwrap_or_default(),
                    bus_id: bus_id__.unwrap_or_default(),
                    placement: placement__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.PrimitiveKey", FIELDS, GeneratedVisitor)
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
        if !self.source_id.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Provenance", len)?;
        if !self.source_file.is_empty() {
            struct_ser.serialize_field("sourceFile", &self.source_file)?;
        }
        if !self.source_id.is_empty() {
            struct_ser.serialize_field("sourceId", &self.source_id)?;
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
            "source_id",
            "sourceId",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            SourceFile,
            SourceId,
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
                            "sourceId" | "source_id" => Ok(GeneratedField::SourceId),
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
                formatter.write_str("struct agni.v1.geom.Provenance")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Provenance, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut source_file__ = None;
                let mut source_id__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::SourceFile => {
                            if source_file__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sourceFile"));
                            }
                            source_file__ = Some(map_.next_value()?);
                        }
                        GeneratedField::SourceId => {
                            if source_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sourceId"));
                            }
                            source_id__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Provenance {
                    source_file: source_file__.unwrap_or_default(),
                    source_id: source_id__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Provenance", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SchematicGeometry {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.design_ref.is_empty() {
            len += 1;
        }
        if self.unit_nm != 0 {
            len += 1;
        }
        if !self.symbols.is_empty() {
            len += 1;
        }
        if !self.sheets.is_empty() {
            len += 1;
        }
        if !self.undrawn.is_empty() {
            len += 1;
        }
        if !self.back_annotated.is_empty() {
            len += 1;
        }
        if !self.text_markup.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.SchematicGeometry", len)?;
        if !self.design_ref.is_empty() {
            struct_ser.serialize_field("designRef", &self.design_ref)?;
        }
        if self.unit_nm != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("unitNm", ToString::to_string(&self.unit_nm).as_str())?;
        }
        if !self.symbols.is_empty() {
            struct_ser.serialize_field("symbols", &self.symbols)?;
        }
        if !self.sheets.is_empty() {
            struct_ser.serialize_field("sheets", &self.sheets)?;
        }
        if !self.undrawn.is_empty() {
            struct_ser.serialize_field("undrawn", &self.undrawn)?;
        }
        if !self.back_annotated.is_empty() {
            struct_ser.serialize_field("backAnnotated", &self.back_annotated)?;
        }
        if !self.text_markup.is_empty() {
            struct_ser.serialize_field("textMarkup", &self.text_markup)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SchematicGeometry {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "design_ref",
            "designRef",
            "unit_nm",
            "unitNm",
            "symbols",
            "sheets",
            "undrawn",
            "back_annotated",
            "backAnnotated",
            "text_markup",
            "textMarkup",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            DesignRef,
            UnitNm,
            Symbols,
            Sheets,
            Undrawn,
            BackAnnotated,
            TextMarkup,
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
                            "designRef" | "design_ref" => Ok(GeneratedField::DesignRef),
                            "unitNm" | "unit_nm" => Ok(GeneratedField::UnitNm),
                            "symbols" => Ok(GeneratedField::Symbols),
                            "sheets" => Ok(GeneratedField::Sheets),
                            "undrawn" => Ok(GeneratedField::Undrawn),
                            "backAnnotated" | "back_annotated" => Ok(GeneratedField::BackAnnotated),
                            "textMarkup" | "text_markup" => Ok(GeneratedField::TextMarkup),
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
            type Value = SchematicGeometry;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.SchematicGeometry")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SchematicGeometry, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut design_ref__ = None;
                let mut unit_nm__ = None;
                let mut symbols__ = None;
                let mut sheets__ = None;
                let mut undrawn__ = None;
                let mut back_annotated__ = None;
                let mut text_markup__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::DesignRef => {
                            if design_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("designRef"));
                            }
                            design_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::UnitNm => {
                            if unit_nm__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unitNm"));
                            }
                            unit_nm__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Symbols => {
                            if symbols__.is_some() {
                                return Err(serde::de::Error::duplicate_field("symbols"));
                            }
                            symbols__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Sheets => {
                            if sheets__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sheets"));
                            }
                            sheets__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Undrawn => {
                            if undrawn__.is_some() {
                                return Err(serde::de::Error::duplicate_field("undrawn"));
                            }
                            undrawn__ = Some(map_.next_value()?);
                        }
                        GeneratedField::BackAnnotated => {
                            if back_annotated__.is_some() {
                                return Err(serde::de::Error::duplicate_field("backAnnotated"));
                            }
                            back_annotated__ = Some(map_.next_value()?);
                        }
                        GeneratedField::TextMarkup => {
                            if text_markup__.is_some() {
                                return Err(serde::de::Error::duplicate_field("textMarkup"));
                            }
                            text_markup__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(SchematicGeometry {
                    design_ref: design_ref__.unwrap_or_default(),
                    unit_nm: unit_nm__.unwrap_or_default(),
                    symbols: symbols__.unwrap_or_default(),
                    sheets: sheets__.unwrap_or_default(),
                    undrawn: undrawn__.unwrap_or_default(),
                    back_annotated: back_annotated__.unwrap_or_default(),
                    text_markup: text_markup__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.SchematicGeometry", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Shape {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.kind != 0 {
            len += 1;
        }
        if !self.points.is_empty() {
            len += 1;
        }
        if self.radius != 0 {
            len += 1;
        }
        if !self.figure_group.is_empty() {
            len += 1;
        }
        if self.fill != 0 {
            len += 1;
        }
        if !self.fill_color.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Shape", len)?;
        if self.kind != 0 {
            let v = shape::Kind::try_from(self.kind)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.kind)))?;
            struct_ser.serialize_field("kind", &v)?;
        }
        if !self.points.is_empty() {
            struct_ser.serialize_field("points", &self.points)?;
        }
        if self.radius != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("radius", ToString::to_string(&self.radius).as_str())?;
        }
        if !self.figure_group.is_empty() {
            struct_ser.serialize_field("figureGroup", &self.figure_group)?;
        }
        if self.fill != 0 {
            let v = shape::Fill::try_from(self.fill)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.fill)))?;
            struct_ser.serialize_field("fill", &v)?;
        }
        if !self.fill_color.is_empty() {
            struct_ser.serialize_field("fillColor", &self.fill_color)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Shape {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "kind",
            "points",
            "radius",
            "figure_group",
            "figureGroup",
            "fill",
            "fill_color",
            "fillColor",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Kind,
            Points,
            Radius,
            FigureGroup,
            Fill,
            FillColor,
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
                            "points" => Ok(GeneratedField::Points),
                            "radius" => Ok(GeneratedField::Radius),
                            "figureGroup" | "figure_group" => Ok(GeneratedField::FigureGroup),
                            "fill" => Ok(GeneratedField::Fill),
                            "fillColor" | "fill_color" => Ok(GeneratedField::FillColor),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Shape;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.Shape")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Shape, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut kind__ = None;
                let mut points__ = None;
                let mut radius__ = None;
                let mut figure_group__ = None;
                let mut fill__ = None;
                let mut fill_color__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value::<shape::Kind>()? as i32);
                        }
                        GeneratedField::Points => {
                            if points__.is_some() {
                                return Err(serde::de::Error::duplicate_field("points"));
                            }
                            points__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Radius => {
                            if radius__.is_some() {
                                return Err(serde::de::Error::duplicate_field("radius"));
                            }
                            radius__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::FigureGroup => {
                            if figure_group__.is_some() {
                                return Err(serde::de::Error::duplicate_field("figureGroup"));
                            }
                            figure_group__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Fill => {
                            if fill__.is_some() {
                                return Err(serde::de::Error::duplicate_field("fill"));
                            }
                            fill__ = Some(map_.next_value::<shape::Fill>()? as i32);
                        }
                        GeneratedField::FillColor => {
                            if fill_color__.is_some() {
                                return Err(serde::de::Error::duplicate_field("fillColor"));
                            }
                            fill_color__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Shape {
                    kind: kind__.unwrap_or_default(),
                    points: points__.unwrap_or_default(),
                    radius: radius__.unwrap_or_default(),
                    figure_group: figure_group__.unwrap_or_default(),
                    fill: fill__.unwrap_or_default(),
                    fill_color: fill_color__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Shape", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for shape::Fill {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "FILL_UNSPECIFIED",
            Self::Outline => "FILL_OUTLINE",
            Self::Background => "FILL_BACKGROUND",
            Self::Color => "FILL_COLOR",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for shape::Fill {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "FILL_UNSPECIFIED",
            "FILL_OUTLINE",
            "FILL_BACKGROUND",
            "FILL_COLOR",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = shape::Fill;

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
                    "FILL_UNSPECIFIED" => Ok(shape::Fill::Unspecified),
                    "FILL_OUTLINE" => Ok(shape::Fill::Outline),
                    "FILL_BACKGROUND" => Ok(shape::Fill::Background),
                    "FILL_COLOR" => Ok(shape::Fill::Color),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for shape::Kind {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "KIND_UNSPECIFIED",
            Self::Polyline => "KIND_POLYLINE",
            Self::Rect => "KIND_RECT",
            Self::Circle => "KIND_CIRCLE",
            Self::Arc => "KIND_ARC",
            Self::Dot => "KIND_DOT",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for shape::Kind {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "KIND_UNSPECIFIED",
            "KIND_POLYLINE",
            "KIND_RECT",
            "KIND_CIRCLE",
            "KIND_ARC",
            "KIND_DOT",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = shape::Kind;

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
                    "KIND_UNSPECIFIED" => Ok(shape::Kind::Unspecified),
                    "KIND_POLYLINE" => Ok(shape::Kind::Polyline),
                    "KIND_RECT" => Ok(shape::Kind::Rect),
                    "KIND_CIRCLE" => Ok(shape::Kind::Circle),
                    "KIND_ARC" => Ok(shape::Kind::Arc),
                    "KIND_DOT" => Ok(shape::Kind::Dot),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for SheetGeometry {
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
        if !self.parent_id.is_empty() {
            len += 1;
        }
        if self.size.is_some() {
            len += 1;
        }
        if !self.placements.is_empty() {
            len += 1;
        }
        if !self.wires.is_empty() {
            len += 1;
        }
        if !self.labels.is_empty() {
            len += 1;
        }
        if !self.shapes.is_empty() {
            len += 1;
        }
        if self.title_block.is_some() {
            len += 1;
        }
        if !self.images.is_empty() {
            len += 1;
        }
        if self.suppress_worksheet {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.SheetGeometry", len)?;
        if !self.id.is_empty() {
            struct_ser.serialize_field("id", &self.id)?;
        }
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.parent_id.is_empty() {
            struct_ser.serialize_field("parentId", &self.parent_id)?;
        }
        if let Some(v) = self.size.as_ref() {
            struct_ser.serialize_field("size", v)?;
        }
        if !self.placements.is_empty() {
            struct_ser.serialize_field("placements", &self.placements)?;
        }
        if !self.wires.is_empty() {
            struct_ser.serialize_field("wires", &self.wires)?;
        }
        if !self.labels.is_empty() {
            struct_ser.serialize_field("labels", &self.labels)?;
        }
        if !self.shapes.is_empty() {
            struct_ser.serialize_field("shapes", &self.shapes)?;
        }
        if let Some(v) = self.title_block.as_ref() {
            struct_ser.serialize_field("titleBlock", v)?;
        }
        if !self.images.is_empty() {
            struct_ser.serialize_field("images", &self.images)?;
        }
        if self.suppress_worksheet {
            struct_ser.serialize_field("suppressWorksheet", &self.suppress_worksheet)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SheetGeometry {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "id",
            "name",
            "parent_id",
            "parentId",
            "size",
            "placements",
            "wires",
            "labels",
            "shapes",
            "title_block",
            "titleBlock",
            "images",
            "suppress_worksheet",
            "suppressWorksheet",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Id,
            Name,
            ParentId,
            Size,
            Placements,
            Wires,
            Labels,
            Shapes,
            TitleBlock,
            Images,
            SuppressWorksheet,
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
                            "parentId" | "parent_id" => Ok(GeneratedField::ParentId),
                            "size" => Ok(GeneratedField::Size),
                            "placements" => Ok(GeneratedField::Placements),
                            "wires" => Ok(GeneratedField::Wires),
                            "labels" => Ok(GeneratedField::Labels),
                            "shapes" => Ok(GeneratedField::Shapes),
                            "titleBlock" | "title_block" => Ok(GeneratedField::TitleBlock),
                            "images" => Ok(GeneratedField::Images),
                            "suppressWorksheet" | "suppress_worksheet" => Ok(GeneratedField::SuppressWorksheet),
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
            type Value = SheetGeometry;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.SheetGeometry")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SheetGeometry, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut id__ = None;
                let mut name__ = None;
                let mut parent_id__ = None;
                let mut size__ = None;
                let mut placements__ = None;
                let mut wires__ = None;
                let mut labels__ = None;
                let mut shapes__ = None;
                let mut title_block__ = None;
                let mut images__ = None;
                let mut suppress_worksheet__ = None;
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
                        GeneratedField::ParentId => {
                            if parent_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("parentId"));
                            }
                            parent_id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Size => {
                            if size__.is_some() {
                                return Err(serde::de::Error::duplicate_field("size"));
                            }
                            size__ = map_.next_value()?;
                        }
                        GeneratedField::Placements => {
                            if placements__.is_some() {
                                return Err(serde::de::Error::duplicate_field("placements"));
                            }
                            placements__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Wires => {
                            if wires__.is_some() {
                                return Err(serde::de::Error::duplicate_field("wires"));
                            }
                            wires__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Labels => {
                            if labels__.is_some() {
                                return Err(serde::de::Error::duplicate_field("labels"));
                            }
                            labels__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Shapes => {
                            if shapes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("shapes"));
                            }
                            shapes__ = Some(map_.next_value()?);
                        }
                        GeneratedField::TitleBlock => {
                            if title_block__.is_some() {
                                return Err(serde::de::Error::duplicate_field("titleBlock"));
                            }
                            title_block__ = map_.next_value()?;
                        }
                        GeneratedField::Images => {
                            if images__.is_some() {
                                return Err(serde::de::Error::duplicate_field("images"));
                            }
                            images__ = Some(map_.next_value()?);
                        }
                        GeneratedField::SuppressWorksheet => {
                            if suppress_worksheet__.is_some() {
                                return Err(serde::de::Error::duplicate_field("suppressWorksheet"));
                            }
                            suppress_worksheet__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(SheetGeometry {
                    id: id__.unwrap_or_default(),
                    name: name__.unwrap_or_default(),
                    parent_id: parent_id__.unwrap_or_default(),
                    size: size__,
                    placements: placements__.unwrap_or_default(),
                    wires: wires__.unwrap_or_default(),
                    labels: labels__.unwrap_or_default(),
                    shapes: shapes__.unwrap_or_default(),
                    title_block: title_block__,
                    images: images__.unwrap_or_default(),
                    suppress_worksheet: suppress_worksheet__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.SheetGeometry", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for StackLayer {
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
        if !self.layer.is_empty() {
            len += 1;
        }
        if self.thickness.is_some() {
            len += 1;
        }
        if !self.material.is_empty() {
            len += 1;
        }
        if !self.dielectric_type.is_empty() {
            len += 1;
        }
        if self.epsilon_r.is_some() {
            len += 1;
        }
        if self.loss_tangent.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.StackLayer", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if !self.layer.is_empty() {
            struct_ser.serialize_field("layer", &self.layer)?;
        }
        if let Some(v) = self.thickness.as_ref() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("thickness", ToString::to_string(&v).as_str())?;
        }
        if !self.material.is_empty() {
            struct_ser.serialize_field("material", &self.material)?;
        }
        if !self.dielectric_type.is_empty() {
            struct_ser.serialize_field("dielectricType", &self.dielectric_type)?;
        }
        if let Some(v) = self.epsilon_r.as_ref() {
            struct_ser.serialize_field("epsilonR", v)?;
        }
        if let Some(v) = self.loss_tangent.as_ref() {
            struct_ser.serialize_field("lossTangent", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for StackLayer {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "kind",
            "layer",
            "thickness",
            "material",
            "dielectric_type",
            "dielectricType",
            "epsilon_r",
            "epsilonR",
            "loss_tangent",
            "lossTangent",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Kind,
            Layer,
            Thickness,
            Material,
            DielectricType,
            EpsilonR,
            LossTangent,
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
                            "layer" => Ok(GeneratedField::Layer),
                            "thickness" => Ok(GeneratedField::Thickness),
                            "material" => Ok(GeneratedField::Material),
                            "dielectricType" | "dielectric_type" => Ok(GeneratedField::DielectricType),
                            "epsilonR" | "epsilon_r" => Ok(GeneratedField::EpsilonR),
                            "lossTangent" | "loss_tangent" => Ok(GeneratedField::LossTangent),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = StackLayer;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.StackLayer")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<StackLayer, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut kind__ = None;
                let mut layer__ = None;
                let mut thickness__ = None;
                let mut material__ = None;
                let mut dielectric_type__ = None;
                let mut epsilon_r__ = None;
                let mut loss_tangent__ = None;
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
                        GeneratedField::Layer => {
                            if layer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layer"));
                            }
                            layer__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Thickness => {
                            if thickness__.is_some() {
                                return Err(serde::de::Error::duplicate_field("thickness"));
                            }
                            thickness__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                        GeneratedField::Material => {
                            if material__.is_some() {
                                return Err(serde::de::Error::duplicate_field("material"));
                            }
                            material__ = Some(map_.next_value()?);
                        }
                        GeneratedField::DielectricType => {
                            if dielectric_type__.is_some() {
                                return Err(serde::de::Error::duplicate_field("dielectricType"));
                            }
                            dielectric_type__ = Some(map_.next_value()?);
                        }
                        GeneratedField::EpsilonR => {
                            if epsilon_r__.is_some() {
                                return Err(serde::de::Error::duplicate_field("epsilonR"));
                            }
                            epsilon_r__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                        GeneratedField::LossTangent => {
                            if loss_tangent__.is_some() {
                                return Err(serde::de::Error::duplicate_field("lossTangent"));
                            }
                            loss_tangent__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                    }
                }
                Ok(StackLayer {
                    name: name__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    layer: layer__.unwrap_or_default(),
                    thickness: thickness__,
                    material: material__.unwrap_or_default(),
                    dielectric_type: dielectric_type__.unwrap_or_default(),
                    epsilon_r: epsilon_r__,
                    loss_tangent: loss_tangent__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.StackLayer", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Stackup {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.layers.is_empty() {
            len += 1;
        }
        if self.thickness.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Stackup", len)?;
        if !self.layers.is_empty() {
            struct_ser.serialize_field("layers", &self.layers)?;
        }
        if let Some(v) = self.thickness.as_ref() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("thickness", ToString::to_string(&v).as_str())?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Stackup {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "layers",
            "thickness",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Layers,
            Thickness,
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
                            "layers" => Ok(GeneratedField::Layers),
                            "thickness" => Ok(GeneratedField::Thickness),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Stackup;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.Stackup")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Stackup, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut layers__ = None;
                let mut thickness__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Layers => {
                            if layers__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layers"));
                            }
                            layers__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Thickness => {
                            if thickness__.is_some() {
                                return Err(serde::de::Error::duplicate_field("thickness"));
                            }
                            thickness__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                    }
                }
                Ok(Stackup {
                    layers: layers__.unwrap_or_default(),
                    thickness: thickness__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Stackup", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SymbolDef {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.cell_ref.is_empty() {
            len += 1;
        }
        if !self.library_ref.is_empty() {
            len += 1;
        }
        if self.bbox.is_some() {
            len += 1;
        }
        if !self.shapes.is_empty() {
            len += 1;
        }
        if !self.pins.is_empty() {
            len += 1;
        }
        if !self.view_ref.is_empty() {
            len += 1;
        }
        if !self.annotations.is_empty() {
            len += 1;
        }
        if self.asset.is_some() {
            len += 1;
        }
        if !self.images.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.SymbolDef", len)?;
        if !self.cell_ref.is_empty() {
            struct_ser.serialize_field("cellRef", &self.cell_ref)?;
        }
        if !self.library_ref.is_empty() {
            struct_ser.serialize_field("libraryRef", &self.library_ref)?;
        }
        if let Some(v) = self.bbox.as_ref() {
            struct_ser.serialize_field("bbox", v)?;
        }
        if !self.shapes.is_empty() {
            struct_ser.serialize_field("shapes", &self.shapes)?;
        }
        if !self.pins.is_empty() {
            struct_ser.serialize_field("pins", &self.pins)?;
        }
        if !self.view_ref.is_empty() {
            struct_ser.serialize_field("viewRef", &self.view_ref)?;
        }
        if !self.annotations.is_empty() {
            struct_ser.serialize_field("annotations", &self.annotations)?;
        }
        if let Some(v) = self.asset.as_ref() {
            struct_ser.serialize_field("asset", v)?;
        }
        if !self.images.is_empty() {
            struct_ser.serialize_field("images", &self.images)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SymbolDef {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "cell_ref",
            "cellRef",
            "library_ref",
            "libraryRef",
            "bbox",
            "shapes",
            "pins",
            "view_ref",
            "viewRef",
            "annotations",
            "asset",
            "images",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            CellRef,
            LibraryRef,
            Bbox,
            Shapes,
            Pins,
            ViewRef,
            Annotations,
            Asset,
            Images,
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
                            "cellRef" | "cell_ref" => Ok(GeneratedField::CellRef),
                            "libraryRef" | "library_ref" => Ok(GeneratedField::LibraryRef),
                            "bbox" => Ok(GeneratedField::Bbox),
                            "shapes" => Ok(GeneratedField::Shapes),
                            "pins" => Ok(GeneratedField::Pins),
                            "viewRef" | "view_ref" => Ok(GeneratedField::ViewRef),
                            "annotations" => Ok(GeneratedField::Annotations),
                            "asset" => Ok(GeneratedField::Asset),
                            "images" => Ok(GeneratedField::Images),
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
            type Value = SymbolDef;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.SymbolDef")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SymbolDef, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut cell_ref__ = None;
                let mut library_ref__ = None;
                let mut bbox__ = None;
                let mut shapes__ = None;
                let mut pins__ = None;
                let mut view_ref__ = None;
                let mut annotations__ = None;
                let mut asset__ = None;
                let mut images__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::CellRef => {
                            if cell_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("cellRef"));
                            }
                            cell_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::LibraryRef => {
                            if library_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("libraryRef"));
                            }
                            library_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Bbox => {
                            if bbox__.is_some() {
                                return Err(serde::de::Error::duplicate_field("bbox"));
                            }
                            bbox__ = map_.next_value()?;
                        }
                        GeneratedField::Shapes => {
                            if shapes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("shapes"));
                            }
                            shapes__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Pins => {
                            if pins__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pins"));
                            }
                            pins__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ViewRef => {
                            if view_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("viewRef"));
                            }
                            view_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Annotations => {
                            if annotations__.is_some() {
                                return Err(serde::de::Error::duplicate_field("annotations"));
                            }
                            annotations__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Asset => {
                            if asset__.is_some() {
                                return Err(serde::de::Error::duplicate_field("asset"));
                            }
                            asset__ = map_.next_value()?;
                        }
                        GeneratedField::Images => {
                            if images__.is_some() {
                                return Err(serde::de::Error::duplicate_field("images"));
                            }
                            images__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(SymbolDef {
                    cell_ref: cell_ref__.unwrap_or_default(),
                    library_ref: library_ref__.unwrap_or_default(),
                    bbox: bbox__,
                    shapes: shapes__.unwrap_or_default(),
                    pins: pins__.unwrap_or_default(),
                    view_ref: view_ref__.unwrap_or_default(),
                    annotations: annotations__.unwrap_or_default(),
                    asset: asset__,
                    images: images__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.SymbolDef", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SymbolPlacement {
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
        if !self.cell_ref.is_empty() {
            len += 1;
        }
        if !self.library_ref.is_empty() {
            len += 1;
        }
        if self.transform.is_some() {
            len += 1;
        }
        if !self.view_ref.is_empty() {
            len += 1;
        }
        if !self.fields.is_empty() {
            len += 1;
        }
        if !self.net_anchor.is_empty() {
            len += 1;
        }
        if self.dnp {
            len += 1;
        }
        if !self.pin_names.is_empty() {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.SymbolPlacement", len)?;
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if !self.cell_ref.is_empty() {
            struct_ser.serialize_field("cellRef", &self.cell_ref)?;
        }
        if !self.library_ref.is_empty() {
            struct_ser.serialize_field("libraryRef", &self.library_ref)?;
        }
        if let Some(v) = self.transform.as_ref() {
            struct_ser.serialize_field("transform", v)?;
        }
        if !self.view_ref.is_empty() {
            struct_ser.serialize_field("viewRef", &self.view_ref)?;
        }
        if !self.fields.is_empty() {
            struct_ser.serialize_field("fields", &self.fields)?;
        }
        if !self.net_anchor.is_empty() {
            struct_ser.serialize_field("netAnchor", &self.net_anchor)?;
        }
        if self.dnp {
            struct_ser.serialize_field("dnp", &self.dnp)?;
        }
        if !self.pin_names.is_empty() {
            struct_ser.serialize_field("pinNames", &self.pin_names)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SymbolPlacement {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "ref_des",
            "refDes",
            "cell_ref",
            "cellRef",
            "library_ref",
            "libraryRef",
            "transform",
            "view_ref",
            "viewRef",
            "fields",
            "net_anchor",
            "netAnchor",
            "dnp",
            "pin_names",
            "pinNames",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            RefDes,
            CellRef,
            LibraryRef,
            Transform,
            ViewRef,
            Fields,
            NetAnchor,
            Dnp,
            PinNames,
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
                            "cellRef" | "cell_ref" => Ok(GeneratedField::CellRef),
                            "libraryRef" | "library_ref" => Ok(GeneratedField::LibraryRef),
                            "transform" => Ok(GeneratedField::Transform),
                            "viewRef" | "view_ref" => Ok(GeneratedField::ViewRef),
                            "fields" => Ok(GeneratedField::Fields),
                            "netAnchor" | "net_anchor" => Ok(GeneratedField::NetAnchor),
                            "dnp" => Ok(GeneratedField::Dnp),
                            "pinNames" | "pin_names" => Ok(GeneratedField::PinNames),
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
            type Value = SymbolPlacement;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.SymbolPlacement")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SymbolPlacement, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut ref_des__ = None;
                let mut cell_ref__ = None;
                let mut library_ref__ = None;
                let mut transform__ = None;
                let mut view_ref__ = None;
                let mut fields__ = None;
                let mut net_anchor__ = None;
                let mut dnp__ = None;
                let mut pin_names__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::CellRef => {
                            if cell_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("cellRef"));
                            }
                            cell_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::LibraryRef => {
                            if library_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("libraryRef"));
                            }
                            library_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Transform => {
                            if transform__.is_some() {
                                return Err(serde::de::Error::duplicate_field("transform"));
                            }
                            transform__ = map_.next_value()?;
                        }
                        GeneratedField::ViewRef => {
                            if view_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("viewRef"));
                            }
                            view_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Fields => {
                            if fields__.is_some() {
                                return Err(serde::de::Error::duplicate_field("fields"));
                            }
                            fields__ = Some(map_.next_value()?);
                        }
                        GeneratedField::NetAnchor => {
                            if net_anchor__.is_some() {
                                return Err(serde::de::Error::duplicate_field("netAnchor"));
                            }
                            net_anchor__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Dnp => {
                            if dnp__.is_some() {
                                return Err(serde::de::Error::duplicate_field("dnp"));
                            }
                            dnp__ = Some(map_.next_value()?);
                        }
                        GeneratedField::PinNames => {
                            if pin_names__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pinNames"));
                            }
                            pin_names__ = Some(
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
                Ok(SymbolPlacement {
                    ref_des: ref_des__.unwrap_or_default(),
                    cell_ref: cell_ref__.unwrap_or_default(),
                    library_ref: library_ref__.unwrap_or_default(),
                    transform: transform__,
                    view_ref: view_ref__.unwrap_or_default(),
                    fields: fields__.unwrap_or_default(),
                    net_anchor: net_anchor__.unwrap_or_default(),
                    dnp: dnp__.unwrap_or_default(),
                    pin_names: pin_names__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.SymbolPlacement", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for TextRun {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.text.is_empty() {
            len += 1;
        }
        if self.kind != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.TextRun", len)?;
        if !self.text.is_empty() {
            struct_ser.serialize_field("text", &self.text)?;
        }
        if self.kind != 0 {
            let v = text_run::Kind::try_from(self.kind)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.kind)))?;
            struct_ser.serialize_field("kind", &v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for TextRun {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "text",
            "kind",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Text,
            Kind,
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
                            "text" => Ok(GeneratedField::Text),
                            "kind" => Ok(GeneratedField::Kind),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = TextRun;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.TextRun")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<TextRun, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut text__ = None;
                let mut kind__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Text => {
                            if text__.is_some() {
                                return Err(serde::de::Error::duplicate_field("text"));
                            }
                            text__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value::<text_run::Kind>()? as i32);
                        }
                    }
                }
                Ok(TextRun {
                    text: text__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.TextRun", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for text_run::Kind {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Plain => "KIND_PLAIN",
            Self::Overbar => "KIND_OVERBAR",
            Self::Subscript => "KIND_SUBSCRIPT",
            Self::Superscript => "KIND_SUPERSCRIPT",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for text_run::Kind {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "KIND_PLAIN",
            "KIND_OVERBAR",
            "KIND_SUBSCRIPT",
            "KIND_SUPERSCRIPT",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = text_run::Kind;

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
                    "KIND_PLAIN" => Ok(text_run::Kind::Plain),
                    "KIND_OVERBAR" => Ok(text_run::Kind::Overbar),
                    "KIND_SUBSCRIPT" => Ok(text_run::Kind::Subscript),
                    "KIND_SUPERSCRIPT" => Ok(text_run::Kind::Superscript),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for TitleBlock {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.title.is_empty() {
            len += 1;
        }
        if !self.rev.is_empty() {
            len += 1;
        }
        if !self.date.is_empty() {
            len += 1;
        }
        if !self.company.is_empty() {
            len += 1;
        }
        if !self.comments.is_empty() {
            len += 1;
        }
        if !self.extra_fields.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.TitleBlock", len)?;
        if !self.title.is_empty() {
            struct_ser.serialize_field("title", &self.title)?;
        }
        if !self.rev.is_empty() {
            struct_ser.serialize_field("rev", &self.rev)?;
        }
        if !self.date.is_empty() {
            struct_ser.serialize_field("date", &self.date)?;
        }
        if !self.company.is_empty() {
            struct_ser.serialize_field("company", &self.company)?;
        }
        if !self.comments.is_empty() {
            struct_ser.serialize_field("comments", &self.comments)?;
        }
        if !self.extra_fields.is_empty() {
            struct_ser.serialize_field("extraFields", &self.extra_fields)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for TitleBlock {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "title",
            "rev",
            "date",
            "company",
            "comments",
            "extra_fields",
            "extraFields",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Title,
            Rev,
            Date,
            Company,
            Comments,
            ExtraFields,
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
                            "title" => Ok(GeneratedField::Title),
                            "rev" => Ok(GeneratedField::Rev),
                            "date" => Ok(GeneratedField::Date),
                            "company" => Ok(GeneratedField::Company),
                            "comments" => Ok(GeneratedField::Comments),
                            "extraFields" | "extra_fields" => Ok(GeneratedField::ExtraFields),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = TitleBlock;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.TitleBlock")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<TitleBlock, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut title__ = None;
                let mut rev__ = None;
                let mut date__ = None;
                let mut company__ = None;
                let mut comments__ = None;
                let mut extra_fields__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Title => {
                            if title__.is_some() {
                                return Err(serde::de::Error::duplicate_field("title"));
                            }
                            title__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Rev => {
                            if rev__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rev"));
                            }
                            rev__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Date => {
                            if date__.is_some() {
                                return Err(serde::de::Error::duplicate_field("date"));
                            }
                            date__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Company => {
                            if company__.is_some() {
                                return Err(serde::de::Error::duplicate_field("company"));
                            }
                            company__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Comments => {
                            if comments__.is_some() {
                                return Err(serde::de::Error::duplicate_field("comments"));
                            }
                            comments__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ExtraFields => {
                            if extra_fields__.is_some() {
                                return Err(serde::de::Error::duplicate_field("extraFields"));
                            }
                            extra_fields__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(TitleBlock {
                    title: title__.unwrap_or_default(),
                    rev: rev__.unwrap_or_default(),
                    date: date__.unwrap_or_default(),
                    company: company__.unwrap_or_default(),
                    comments: comments__.unwrap_or_default(),
                    extra_fields: extra_fields__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.TitleBlock", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for TrackSegment {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.a.is_some() {
            len += 1;
        }
        if self.b.is_some() {
            len += 1;
        }
        if self.width != 0 {
            len += 1;
        }
        if !self.layer.is_empty() {
            len += 1;
        }
        if self.mid.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.TrackSegment", len)?;
        if let Some(v) = self.a.as_ref() {
            struct_ser.serialize_field("a", v)?;
        }
        if let Some(v) = self.b.as_ref() {
            struct_ser.serialize_field("b", v)?;
        }
        if self.width != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("width", ToString::to_string(&self.width).as_str())?;
        }
        if !self.layer.is_empty() {
            struct_ser.serialize_field("layer", &self.layer)?;
        }
        if let Some(v) = self.mid.as_ref() {
            struct_ser.serialize_field("mid", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for TrackSegment {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "a",
            "b",
            "width",
            "layer",
            "mid",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            A,
            B,
            Width,
            Layer,
            Mid,
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
                            "width" => Ok(GeneratedField::Width),
                            "layer" => Ok(GeneratedField::Layer),
                            "mid" => Ok(GeneratedField::Mid),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = TrackSegment;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.TrackSegment")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<TrackSegment, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut a__ = None;
                let mut b__ = None;
                let mut width__ = None;
                let mut layer__ = None;
                let mut mid__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::A => {
                            if a__.is_some() {
                                return Err(serde::de::Error::duplicate_field("a"));
                            }
                            a__ = map_.next_value()?;
                        }
                        GeneratedField::B => {
                            if b__.is_some() {
                                return Err(serde::de::Error::duplicate_field("b"));
                            }
                            b__ = map_.next_value()?;
                        }
                        GeneratedField::Width => {
                            if width__.is_some() {
                                return Err(serde::de::Error::duplicate_field("width"));
                            }
                            width__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Layer => {
                            if layer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layer"));
                            }
                            layer__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Mid => {
                            if mid__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mid"));
                            }
                            mid__ = map_.next_value()?;
                        }
                    }
                }
                Ok(TrackSegment {
                    a: a__,
                    b: b__,
                    width: width__.unwrap_or_default(),
                    layer: layer__.unwrap_or_default(),
                    mid: mid__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.TrackSegment", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Transform {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.origin.is_some() {
            len += 1;
        }
        if self.rotation_deg != 0 {
            len += 1;
        }
        if self.mirror_x {
            len += 1;
        }
        if self.mirror_y {
            len += 1;
        }
        if self.scale_x != 0. {
            len += 1;
        }
        if self.scale_y != 0. {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Transform", len)?;
        if let Some(v) = self.origin.as_ref() {
            struct_ser.serialize_field("origin", v)?;
        }
        if self.rotation_deg != 0 {
            struct_ser.serialize_field("rotationDeg", &self.rotation_deg)?;
        }
        if self.mirror_x {
            struct_ser.serialize_field("mirrorX", &self.mirror_x)?;
        }
        if self.mirror_y {
            struct_ser.serialize_field("mirrorY", &self.mirror_y)?;
        }
        if self.scale_x != 0. {
            struct_ser.serialize_field("scaleX", &self.scale_x)?;
        }
        if self.scale_y != 0. {
            struct_ser.serialize_field("scaleY", &self.scale_y)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Transform {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "origin",
            "rotation_deg",
            "rotationDeg",
            "mirror_x",
            "mirrorX",
            "mirror_y",
            "mirrorY",
            "scale_x",
            "scaleX",
            "scale_y",
            "scaleY",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Origin,
            RotationDeg,
            MirrorX,
            MirrorY,
            ScaleX,
            ScaleY,
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
                            "origin" => Ok(GeneratedField::Origin),
                            "rotationDeg" | "rotation_deg" => Ok(GeneratedField::RotationDeg),
                            "mirrorX" | "mirror_x" => Ok(GeneratedField::MirrorX),
                            "mirrorY" | "mirror_y" => Ok(GeneratedField::MirrorY),
                            "scaleX" | "scale_x" => Ok(GeneratedField::ScaleX),
                            "scaleY" | "scale_y" => Ok(GeneratedField::ScaleY),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Transform;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.Transform")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Transform, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut origin__ = None;
                let mut rotation_deg__ = None;
                let mut mirror_x__ = None;
                let mut mirror_y__ = None;
                let mut scale_x__ = None;
                let mut scale_y__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Origin => {
                            if origin__.is_some() {
                                return Err(serde::de::Error::duplicate_field("origin"));
                            }
                            origin__ = map_.next_value()?;
                        }
                        GeneratedField::RotationDeg => {
                            if rotation_deg__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rotationDeg"));
                            }
                            rotation_deg__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::MirrorX => {
                            if mirror_x__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mirrorX"));
                            }
                            mirror_x__ = Some(map_.next_value()?);
                        }
                        GeneratedField::MirrorY => {
                            if mirror_y__.is_some() {
                                return Err(serde::de::Error::duplicate_field("mirrorY"));
                            }
                            mirror_y__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ScaleX => {
                            if scale_x__.is_some() {
                                return Err(serde::de::Error::duplicate_field("scaleX"));
                            }
                            scale_x__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::ScaleY => {
                            if scale_y__.is_some() {
                                return Err(serde::de::Error::duplicate_field("scaleY"));
                            }
                            scale_y__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(Transform {
                    origin: origin__,
                    rotation_deg: rotation_deg__.unwrap_or_default(),
                    mirror_x: mirror_x__.unwrap_or_default(),
                    mirror_y: mirror_y__.unwrap_or_default(),
                    scale_x: scale_x__.unwrap_or_default(),
                    scale_y: scale_y__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Transform", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for UndrawnPlacement {
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
        if !self.cell_ref.is_empty() {
            len += 1;
        }
        if !self.library_ref.is_empty() {
            len += 1;
        }
        if !self.sheet_id.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.UndrawnPlacement", len)?;
        if !self.ref_des.is_empty() {
            struct_ser.serialize_field("refDes", &self.ref_des)?;
        }
        if !self.cell_ref.is_empty() {
            struct_ser.serialize_field("cellRef", &self.cell_ref)?;
        }
        if !self.library_ref.is_empty() {
            struct_ser.serialize_field("libraryRef", &self.library_ref)?;
        }
        if !self.sheet_id.is_empty() {
            struct_ser.serialize_field("sheetId", &self.sheet_id)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for UndrawnPlacement {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "ref_des",
            "refDes",
            "cell_ref",
            "cellRef",
            "library_ref",
            "libraryRef",
            "sheet_id",
            "sheetId",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            RefDes,
            CellRef,
            LibraryRef,
            SheetId,
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
                            "cellRef" | "cell_ref" => Ok(GeneratedField::CellRef),
                            "libraryRef" | "library_ref" => Ok(GeneratedField::LibraryRef),
                            "sheetId" | "sheet_id" => Ok(GeneratedField::SheetId),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = UndrawnPlacement;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.UndrawnPlacement")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<UndrawnPlacement, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut ref_des__ = None;
                let mut cell_ref__ = None;
                let mut library_ref__ = None;
                let mut sheet_id__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::RefDes => {
                            if ref_des__.is_some() {
                                return Err(serde::de::Error::duplicate_field("refDes"));
                            }
                            ref_des__ = Some(map_.next_value()?);
                        }
                        GeneratedField::CellRef => {
                            if cell_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("cellRef"));
                            }
                            cell_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::LibraryRef => {
                            if library_ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("libraryRef"));
                            }
                            library_ref__ = Some(map_.next_value()?);
                        }
                        GeneratedField::SheetId => {
                            if sheet_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sheetId"));
                            }
                            sheet_id__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(UndrawnPlacement {
                    ref_des: ref_des__.unwrap_or_default(),
                    cell_ref: cell_ref__.unwrap_or_default(),
                    library_ref: library_ref__.unwrap_or_default(),
                    sheet_id: sheet_id__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.UndrawnPlacement", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Via {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.at.is_some() {
            len += 1;
        }
        if self.size != 0 {
            len += 1;
        }
        if self.drill != 0 {
            len += 1;
        }
        if !self.layer_from.is_empty() {
            len += 1;
        }
        if !self.layer_to.is_empty() {
            len += 1;
        }
        if !self.padstack.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Via", len)?;
        if let Some(v) = self.at.as_ref() {
            struct_ser.serialize_field("at", v)?;
        }
        if self.size != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("size", ToString::to_string(&self.size).as_str())?;
        }
        if self.drill != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("drill", ToString::to_string(&self.drill).as_str())?;
        }
        if !self.layer_from.is_empty() {
            struct_ser.serialize_field("layerFrom", &self.layer_from)?;
        }
        if !self.layer_to.is_empty() {
            struct_ser.serialize_field("layerTo", &self.layer_to)?;
        }
        if !self.padstack.is_empty() {
            struct_ser.serialize_field("padstack", &self.padstack)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Via {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "at",
            "size",
            "drill",
            "layer_from",
            "layerFrom",
            "layer_to",
            "layerTo",
            "padstack",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            At,
            Size,
            Drill,
            LayerFrom,
            LayerTo,
            Padstack,
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
                            "at" => Ok(GeneratedField::At),
                            "size" => Ok(GeneratedField::Size),
                            "drill" => Ok(GeneratedField::Drill),
                            "layerFrom" | "layer_from" => Ok(GeneratedField::LayerFrom),
                            "layerTo" | "layer_to" => Ok(GeneratedField::LayerTo),
                            "padstack" => Ok(GeneratedField::Padstack),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Via;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.Via")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Via, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut at__ = None;
                let mut size__ = None;
                let mut drill__ = None;
                let mut layer_from__ = None;
                let mut layer_to__ = None;
                let mut padstack__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::At => {
                            if at__.is_some() {
                                return Err(serde::de::Error::duplicate_field("at"));
                            }
                            at__ = map_.next_value()?;
                        }
                        GeneratedField::Size => {
                            if size__.is_some() {
                                return Err(serde::de::Error::duplicate_field("size"));
                            }
                            size__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Drill => {
                            if drill__.is_some() {
                                return Err(serde::de::Error::duplicate_field("drill"));
                            }
                            drill__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::LayerFrom => {
                            if layer_from__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layerFrom"));
                            }
                            layer_from__ = Some(map_.next_value()?);
                        }
                        GeneratedField::LayerTo => {
                            if layer_to__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layerTo"));
                            }
                            layer_to__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Padstack => {
                            if padstack__.is_some() {
                                return Err(serde::de::Error::duplicate_field("padstack"));
                            }
                            padstack__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Via {
                    at: at__,
                    size: size__.unwrap_or_default(),
                    drill: drill__.unwrap_or_default(),
                    layer_from: layer_from__.unwrap_or_default(),
                    layer_to: layer_to__.unwrap_or_default(),
                    padstack: padstack__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Via", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for WireGeometry {
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
        if !self.net_id.is_empty() {
            len += 1;
        }
        if !self.polylines.is_empty() {
            len += 1;
        }
        if self.kind != 0 {
            len += 1;
        }
        if self.prov.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.WireGeometry", len)?;
        if !self.net.is_empty() {
            struct_ser.serialize_field("net", &self.net)?;
        }
        if !self.net_id.is_empty() {
            struct_ser.serialize_field("netId", &self.net_id)?;
        }
        if !self.polylines.is_empty() {
            struct_ser.serialize_field("polylines", &self.polylines)?;
        }
        if self.kind != 0 {
            let v = wire_geometry::Kind::try_from(self.kind)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.kind)))?;
            struct_ser.serialize_field("kind", &v)?;
        }
        if let Some(v) = self.prov.as_ref() {
            struct_ser.serialize_field("prov", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for WireGeometry {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "net",
            "net_id",
            "netId",
            "polylines",
            "kind",
            "prov",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Net,
            NetId,
            Polylines,
            Kind,
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
                            "net" => Ok(GeneratedField::Net),
                            "netId" | "net_id" => Ok(GeneratedField::NetId),
                            "polylines" => Ok(GeneratedField::Polylines),
                            "kind" => Ok(GeneratedField::Kind),
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
            type Value = WireGeometry;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.WireGeometry")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<WireGeometry, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut net__ = None;
                let mut net_id__ = None;
                let mut polylines__ = None;
                let mut kind__ = None;
                let mut prov__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Net => {
                            if net__.is_some() {
                                return Err(serde::de::Error::duplicate_field("net"));
                            }
                            net__ = Some(map_.next_value()?);
                        }
                        GeneratedField::NetId => {
                            if net_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("netId"));
                            }
                            net_id__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Polylines => {
                            if polylines__.is_some() {
                                return Err(serde::de::Error::duplicate_field("polylines"));
                            }
                            polylines__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value::<wire_geometry::Kind>()? as i32);
                        }
                        GeneratedField::Prov => {
                            if prov__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prov"));
                            }
                            prov__ = map_.next_value()?;
                        }
                    }
                }
                Ok(WireGeometry {
                    net: net__.unwrap_or_default(),
                    net_id: net_id__.unwrap_or_default(),
                    polylines: polylines__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    prov: prov__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.WireGeometry", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for wire_geometry::Kind {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "KIND_UNSPECIFIED",
            Self::Wire => "KIND_WIRE",
            Self::Bus => "KIND_BUS",
            Self::BusEntry => "KIND_BUS_ENTRY",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for wire_geometry::Kind {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "KIND_UNSPECIFIED",
            "KIND_WIRE",
            "KIND_BUS",
            "KIND_BUS_ENTRY",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = wire_geometry::Kind;

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
                    "KIND_UNSPECIFIED" => Ok(wire_geometry::Kind::Unspecified),
                    "KIND_WIRE" => Ok(wire_geometry::Kind::Wire),
                    "KIND_BUS" => Ok(wire_geometry::Kind::Bus),
                    "KIND_BUS_ENTRY" => Ok(wire_geometry::Kind::BusEntry),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for Zone {
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
        if !self.layer.is_empty() {
            len += 1;
        }
        if self.outline.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.geom.Zone", len)?;
        if !self.net.is_empty() {
            struct_ser.serialize_field("net", &self.net)?;
        }
        if !self.layer.is_empty() {
            struct_ser.serialize_field("layer", &self.layer)?;
        }
        if let Some(v) = self.outline.as_ref() {
            struct_ser.serialize_field("outline", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Zone {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "net",
            "layer",
            "outline",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Net,
            Layer,
            Outline,
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
                            "layer" => Ok(GeneratedField::Layer),
                            "outline" => Ok(GeneratedField::Outline),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Zone;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.geom.Zone")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Zone, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut net__ = None;
                let mut layer__ = None;
                let mut outline__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Net => {
                            if net__.is_some() {
                                return Err(serde::de::Error::duplicate_field("net"));
                            }
                            net__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Layer => {
                            if layer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("layer"));
                            }
                            layer__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Outline => {
                            if outline__.is_some() {
                                return Err(serde::de::Error::duplicate_field("outline"));
                            }
                            outline__ = map_.next_value()?;
                        }
                    }
                }
                Ok(Zone {
                    net: net__.unwrap_or_default(),
                    layer: layer__.unwrap_or_default(),
                    outline: outline__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.geom.Zone", FIELDS, GeneratedVisitor)
    }
}
