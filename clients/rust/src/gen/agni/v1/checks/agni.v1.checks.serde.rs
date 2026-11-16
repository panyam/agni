// @generated
impl serde::Serialize for CheckReport {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.source.is_empty() {
            len += 1;
        }
        if self.rules_run != 0 {
            len += 1;
        }
        if !self.sections.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.CheckReport", len)?;
        if !self.source.is_empty() {
            struct_ser.serialize_field("source", &self.source)?;
        }
        if self.rules_run != 0 {
            struct_ser.serialize_field("rulesRun", &self.rules_run)?;
        }
        if !self.sections.is_empty() {
            struct_ser.serialize_field("sections", &self.sections)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for CheckReport {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "source",
            "rules_run",
            "rulesRun",
            "sections",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Source,
            RulesRun,
            Sections,
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
                            "source" => Ok(GeneratedField::Source),
                            "rulesRun" | "rules_run" => Ok(GeneratedField::RulesRun),
                            "sections" => Ok(GeneratedField::Sections),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = CheckReport;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.CheckReport")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<CheckReport, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut source__ = None;
                let mut rules_run__ = None;
                let mut sections__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Source => {
                            if source__.is_some() {
                                return Err(serde::de::Error::duplicate_field("source"));
                            }
                            source__ = Some(map_.next_value()?);
                        }
                        GeneratedField::RulesRun => {
                            if rules_run__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rulesRun"));
                            }
                            rules_run__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Sections => {
                            if sections__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sections"));
                            }
                            sections__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(CheckReport {
                    source: source__.unwrap_or_default(),
                    rules_run: rules_run__.unwrap_or_default(),
                    sections: sections__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.CheckReport", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for check_report::RuleGroup {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.rule.is_empty() {
            len += 1;
        }
        if !self.summary.is_empty() {
            len += 1;
        }
        if !self.findings.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.CheckReport.RuleGroup", len)?;
        if !self.rule.is_empty() {
            struct_ser.serialize_field("rule", &self.rule)?;
        }
        if !self.summary.is_empty() {
            struct_ser.serialize_field("summary", &self.summary)?;
        }
        if !self.findings.is_empty() {
            struct_ser.serialize_field("findings", &self.findings)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for check_report::RuleGroup {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "rule",
            "summary",
            "findings",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Rule,
            Summary,
            Findings,
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
                            "rule" => Ok(GeneratedField::Rule),
                            "summary" => Ok(GeneratedField::Summary),
                            "findings" => Ok(GeneratedField::Findings),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = check_report::RuleGroup;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.CheckReport.RuleGroup")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<check_report::RuleGroup, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut rule__ = None;
                let mut summary__ = None;
                let mut findings__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Rule => {
                            if rule__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rule"));
                            }
                            rule__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Summary => {
                            if summary__.is_some() {
                                return Err(serde::de::Error::duplicate_field("summary"));
                            }
                            summary__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Findings => {
                            if findings__.is_some() {
                                return Err(serde::de::Error::duplicate_field("findings"));
                            }
                            findings__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(check_report::RuleGroup {
                    rule: rule__.unwrap_or_default(),
                    summary: summary__.unwrap_or_default(),
                    findings: findings__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.CheckReport.RuleGroup", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for check_report::SeveritySection {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.severity.is_empty() {
            len += 1;
        }
        if self.count != 0 {
            len += 1;
        }
        if !self.rules.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.CheckReport.SeveritySection", len)?;
        if !self.severity.is_empty() {
            struct_ser.serialize_field("severity", &self.severity)?;
        }
        if self.count != 0 {
            struct_ser.serialize_field("count", &self.count)?;
        }
        if !self.rules.is_empty() {
            struct_ser.serialize_field("rules", &self.rules)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for check_report::SeveritySection {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "severity",
            "count",
            "rules",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Severity,
            Count,
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
                            "severity" => Ok(GeneratedField::Severity),
                            "count" => Ok(GeneratedField::Count),
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
            type Value = check_report::SeveritySection;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.CheckReport.SeveritySection")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<check_report::SeveritySection, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut severity__ = None;
                let mut count__ = None;
                let mut rules__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Severity => {
                            if severity__.is_some() {
                                return Err(serde::de::Error::duplicate_field("severity"));
                            }
                            severity__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Count => {
                            if count__.is_some() {
                                return Err(serde::de::Error::duplicate_field("count"));
                            }
                            count__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Rules => {
                            if rules__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rules"));
                            }
                            rules__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(check_report::SeveritySection {
                    severity: severity__.unwrap_or_default(),
                    count: count__.unwrap_or_default(),
                    rules: rules__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.CheckReport.SeveritySection", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for CheckResults {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.meta.is_some() {
            len += 1;
        }
        if self.design.is_some() {
            len += 1;
        }
        if self.run.is_some() {
            len += 1;
        }
        if !self.catalog.is_empty() {
            len += 1;
        }
        if !self.skipped.is_empty() {
            len += 1;
        }
        if !self.findings.is_empty() {
            len += 1;
        }
        if !self.manifest.is_empty() {
            len += 1;
        }
        if !self.areas.is_empty() {
            len += 1;
        }
        if self.import_summary.is_some() {
            len += 1;
        }
        if self.manifest_snapshot.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.CheckResults", len)?;
        if let Some(v) = self.meta.as_ref() {
            struct_ser.serialize_field("meta", v)?;
        }
        if let Some(v) = self.design.as_ref() {
            struct_ser.serialize_field("design", v)?;
        }
        if let Some(v) = self.run.as_ref() {
            struct_ser.serialize_field("run", v)?;
        }
        if !self.catalog.is_empty() {
            struct_ser.serialize_field("catalog", &self.catalog)?;
        }
        if !self.skipped.is_empty() {
            struct_ser.serialize_field("skipped", &self.skipped)?;
        }
        if !self.findings.is_empty() {
            struct_ser.serialize_field("findings", &self.findings)?;
        }
        if !self.manifest.is_empty() {
            struct_ser.serialize_field("manifest", &self.manifest)?;
        }
        if !self.areas.is_empty() {
            struct_ser.serialize_field("areas", &self.areas)?;
        }
        if let Some(v) = self.import_summary.as_ref() {
            struct_ser.serialize_field("importSummary", v)?;
        }
        if let Some(v) = self.manifest_snapshot.as_ref() {
            struct_ser.serialize_field("manifestSnapshot", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for CheckResults {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "meta",
            "design",
            "run",
            "catalog",
            "skipped",
            "findings",
            "manifest",
            "areas",
            "import_summary",
            "importSummary",
            "manifest_snapshot",
            "manifestSnapshot",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Meta,
            Design,
            Run,
            Catalog,
            Skipped,
            Findings,
            Manifest,
            Areas,
            ImportSummary,
            ManifestSnapshot,
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
                            "meta" => Ok(GeneratedField::Meta),
                            "design" => Ok(GeneratedField::Design),
                            "run" => Ok(GeneratedField::Run),
                            "catalog" => Ok(GeneratedField::Catalog),
                            "skipped" => Ok(GeneratedField::Skipped),
                            "findings" => Ok(GeneratedField::Findings),
                            "manifest" => Ok(GeneratedField::Manifest),
                            "areas" => Ok(GeneratedField::Areas),
                            "importSummary" | "import_summary" => Ok(GeneratedField::ImportSummary),
                            "manifestSnapshot" | "manifest_snapshot" => Ok(GeneratedField::ManifestSnapshot),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = CheckResults;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.CheckResults")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<CheckResults, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut meta__ = None;
                let mut design__ = None;
                let mut run__ = None;
                let mut catalog__ = None;
                let mut skipped__ = None;
                let mut findings__ = None;
                let mut manifest__ = None;
                let mut areas__ = None;
                let mut import_summary__ = None;
                let mut manifest_snapshot__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Meta => {
                            if meta__.is_some() {
                                return Err(serde::de::Error::duplicate_field("meta"));
                            }
                            meta__ = map_.next_value()?;
                        }
                        GeneratedField::Design => {
                            if design__.is_some() {
                                return Err(serde::de::Error::duplicate_field("design"));
                            }
                            design__ = map_.next_value()?;
                        }
                        GeneratedField::Run => {
                            if run__.is_some() {
                                return Err(serde::de::Error::duplicate_field("run"));
                            }
                            run__ = map_.next_value()?;
                        }
                        GeneratedField::Catalog => {
                            if catalog__.is_some() {
                                return Err(serde::de::Error::duplicate_field("catalog"));
                            }
                            catalog__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Skipped => {
                            if skipped__.is_some() {
                                return Err(serde::de::Error::duplicate_field("skipped"));
                            }
                            skipped__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Findings => {
                            if findings__.is_some() {
                                return Err(serde::de::Error::duplicate_field("findings"));
                            }
                            findings__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Manifest => {
                            if manifest__.is_some() {
                                return Err(serde::de::Error::duplicate_field("manifest"));
                            }
                            manifest__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Areas => {
                            if areas__.is_some() {
                                return Err(serde::de::Error::duplicate_field("areas"));
                            }
                            areas__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ImportSummary => {
                            if import_summary__.is_some() {
                                return Err(serde::de::Error::duplicate_field("importSummary"));
                            }
                            import_summary__ = map_.next_value()?;
                        }
                        GeneratedField::ManifestSnapshot => {
                            if manifest_snapshot__.is_some() {
                                return Err(serde::de::Error::duplicate_field("manifestSnapshot"));
                            }
                            manifest_snapshot__ = map_.next_value()?;
                        }
                    }
                }
                Ok(CheckResults {
                    meta: meta__,
                    design: design__,
                    run: run__,
                    catalog: catalog__.unwrap_or_default(),
                    skipped: skipped__.unwrap_or_default(),
                    findings: findings__.unwrap_or_default(),
                    manifest: manifest__.unwrap_or_default(),
                    areas: areas__.unwrap_or_default(),
                    import_summary: import_summary__,
                    manifest_snapshot: manifest_snapshot__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.CheckResults", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ContextSubject {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.subject.is_some() {
            len += 1;
        }
        if !self.role.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ContextSubject", len)?;
        if let Some(v) = self.subject.as_ref() {
            struct_ser.serialize_field("subject", v)?;
        }
        if !self.role.is_empty() {
            struct_ser.serialize_field("role", &self.role)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ContextSubject {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "subject",
            "role",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Subject,
            Role,
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
                            "role" => Ok(GeneratedField::Role),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ContextSubject;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ContextSubject")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ContextSubject, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut subject__ = None;
                let mut role__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Subject => {
                            if subject__.is_some() {
                                return Err(serde::de::Error::duplicate_field("subject"));
                            }
                            subject__ = map_.next_value()?;
                        }
                        GeneratedField::Role => {
                            if role__.is_some() {
                                return Err(serde::de::Error::duplicate_field("role"));
                            }
                            role__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ContextSubject {
                    subject: subject__,
                    role: role__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ContextSubject", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ContextVar {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.var.is_empty() {
            len += 1;
        }
        if !self.kind.is_empty() {
            len += 1;
        }
        if !self.role.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ContextVar", len)?;
        if !self.var.is_empty() {
            struct_ser.serialize_field("var", &self.var)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if !self.role.is_empty() {
            struct_ser.serialize_field("role", &self.role)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ContextVar {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "var",
            "kind",
            "role",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Var,
            Kind,
            Role,
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
                            "var" => Ok(GeneratedField::Var),
                            "kind" => Ok(GeneratedField::Kind),
                            "role" => Ok(GeneratedField::Role),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ContextVar;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ContextVar")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ContextVar, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut var__ = None;
                let mut kind__ = None;
                let mut role__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Var => {
                            if var__.is_some() {
                                return Err(serde::de::Error::duplicate_field("var"));
                            }
                            var__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Role => {
                            if role__.is_some() {
                                return Err(serde::de::Error::duplicate_field("role"));
                            }
                            role__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ContextVar {
                    var: var__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    role: role__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ContextVar", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatalogAggregate {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.func.is_empty() {
            len += 1;
        }
        if !self.var.is_empty() {
            len += 1;
        }
        if self.distinct {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatalogAggregate", len)?;
        if !self.func.is_empty() {
            struct_ser.serialize_field("func", &self.func)?;
        }
        if !self.var.is_empty() {
            struct_ser.serialize_field("var", &self.var)?;
        }
        if self.distinct {
            struct_ser.serialize_field("distinct", &self.distinct)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatalogAggregate {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "func",
            "var",
            "distinct",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Func,
            Var,
            Distinct,
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
                            "func" => Ok(GeneratedField::Func),
                            "var" => Ok(GeneratedField::Var),
                            "distinct" => Ok(GeneratedField::Distinct),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatalogAggregate;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatalogAggregate")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatalogAggregate, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut func__ = None;
                let mut var__ = None;
                let mut distinct__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Func => {
                            if func__.is_some() {
                                return Err(serde::de::Error::duplicate_field("func"));
                            }
                            func__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Var => {
                            if var__.is_some() {
                                return Err(serde::de::Error::duplicate_field("var"));
                            }
                            var__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Distinct => {
                            if distinct__.is_some() {
                                return Err(serde::de::Error::duplicate_field("distinct"));
                            }
                            distinct__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(DatalogAggregate {
                    func: func__.unwrap_or_default(),
                    var: var__.unwrap_or_default(),
                    distinct: distinct__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatalogAggregate", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatalogArgType {
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
        if !self.kind_from.is_empty() {
            len += 1;
        }
        if !self.owner.is_empty() {
            len += 1;
        }
        if !self.r#type.is_empty() {
            len += 1;
        }
        if !self.unit.is_empty() {
            len += 1;
        }
        if !self.domain.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatalogArgType", len)?;
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if !self.kind_from.is_empty() {
            struct_ser.serialize_field("kindFrom", &self.kind_from)?;
        }
        if !self.owner.is_empty() {
            struct_ser.serialize_field("owner", &self.owner)?;
        }
        if !self.r#type.is_empty() {
            struct_ser.serialize_field("type", &self.r#type)?;
        }
        if !self.unit.is_empty() {
            struct_ser.serialize_field("unit", &self.unit)?;
        }
        if !self.domain.is_empty() {
            struct_ser.serialize_field("domain", &self.domain)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatalogArgType {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "kind",
            "kind_from",
            "kindFrom",
            "owner",
            "type",
            "unit",
            "domain",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Kind,
            KindFrom,
            Owner,
            Type,
            Unit,
            Domain,
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
                            "kindFrom" | "kind_from" => Ok(GeneratedField::KindFrom),
                            "owner" => Ok(GeneratedField::Owner),
                            "type" => Ok(GeneratedField::Type),
                            "unit" => Ok(GeneratedField::Unit),
                            "domain" => Ok(GeneratedField::Domain),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatalogArgType;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatalogArgType")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatalogArgType, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut kind__ = None;
                let mut kind_from__ = None;
                let mut owner__ = None;
                let mut r#type__ = None;
                let mut unit__ = None;
                let mut domain__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::KindFrom => {
                            if kind_from__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kindFrom"));
                            }
                            kind_from__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Owner => {
                            if owner__.is_some() {
                                return Err(serde::de::Error::duplicate_field("owner"));
                            }
                            owner__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Type => {
                            if r#type__.is_some() {
                                return Err(serde::de::Error::duplicate_field("type"));
                            }
                            r#type__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Unit => {
                            if unit__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unit"));
                            }
                            unit__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Domain => {
                            if domain__.is_some() {
                                return Err(serde::de::Error::duplicate_field("domain"));
                            }
                            domain__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(DatalogArgType {
                    kind: kind__.unwrap_or_default(),
                    kind_from: kind_from__.unwrap_or_default(),
                    owner: owner__.unwrap_or_default(),
                    r#type: r#type__.unwrap_or_default(),
                    unit: unit__.unwrap_or_default(),
                    domain: domain__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatalogArgType", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatalogAtom {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.relation.is_empty() {
            len += 1;
        }
        if !self.args.is_empty() {
            len += 1;
        }
        if !self.fact.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatalogAtom", len)?;
        if !self.relation.is_empty() {
            struct_ser.serialize_field("relation", &self.relation)?;
        }
        if !self.args.is_empty() {
            struct_ser.serialize_field("args", &self.args)?;
        }
        if !self.fact.is_empty() {
            struct_ser.serialize_field("fact", &self.fact)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatalogAtom {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "relation",
            "args",
            "fact",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Relation,
            Args,
            Fact,
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
                            "relation" => Ok(GeneratedField::Relation),
                            "args" => Ok(GeneratedField::Args),
                            "fact" => Ok(GeneratedField::Fact),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatalogAtom;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatalogAtom")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatalogAtom, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut relation__ = None;
                let mut args__ = None;
                let mut fact__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Relation => {
                            if relation__.is_some() {
                                return Err(serde::de::Error::duplicate_field("relation"));
                            }
                            relation__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Args => {
                            if args__.is_some() {
                                return Err(serde::de::Error::duplicate_field("args"));
                            }
                            args__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Fact => {
                            if fact__.is_some() {
                                return Err(serde::de::Error::duplicate_field("fact"));
                            }
                            fact__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(DatalogAtom {
                    relation: relation__.unwrap_or_default(),
                    args: args__.unwrap_or_default(),
                    fact: fact__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatalogAtom", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatalogBody {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.literals.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatalogBody", len)?;
        if !self.literals.is_empty() {
            struct_ser.serialize_field("literals", &self.literals)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatalogBody {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "literals",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Literals,
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
                            "literals" => Ok(GeneratedField::Literals),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatalogBody;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatalogBody")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatalogBody, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut literals__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Literals => {
                            if literals__.is_some() {
                                return Err(serde::de::Error::duplicate_field("literals"));
                            }
                            literals__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(DatalogBody {
                    literals: literals__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatalogBody", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatalogBodyAggregate {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.result.is_empty() {
            len += 1;
        }
        if self.agg.is_some() {
            len += 1;
        }
        if self.body.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatalogBodyAggregate", len)?;
        if !self.result.is_empty() {
            struct_ser.serialize_field("result", &self.result)?;
        }
        if let Some(v) = self.agg.as_ref() {
            struct_ser.serialize_field("agg", v)?;
        }
        if let Some(v) = self.body.as_ref() {
            struct_ser.serialize_field("body", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatalogBodyAggregate {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "result",
            "agg",
            "body",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Result,
            Agg,
            Body,
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
                            "result" => Ok(GeneratedField::Result),
                            "agg" => Ok(GeneratedField::Agg),
                            "body" => Ok(GeneratedField::Body),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatalogBodyAggregate;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatalogBodyAggregate")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatalogBodyAggregate, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut result__ = None;
                let mut agg__ = None;
                let mut body__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Result => {
                            if result__.is_some() {
                                return Err(serde::de::Error::duplicate_field("result"));
                            }
                            result__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Agg => {
                            if agg__.is_some() {
                                return Err(serde::de::Error::duplicate_field("agg"));
                            }
                            agg__ = map_.next_value()?;
                        }
                        GeneratedField::Body => {
                            if body__.is_some() {
                                return Err(serde::de::Error::duplicate_field("body"));
                            }
                            body__ = map_.next_value()?;
                        }
                    }
                }
                Ok(DatalogBodyAggregate {
                    result: result__.unwrap_or_default(),
                    agg: agg__,
                    body: body__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatalogBodyAggregate", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatalogCompare {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.left.is_some() {
            len += 1;
        }
        if !self.op.is_empty() {
            len += 1;
        }
        if self.right.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatalogCompare", len)?;
        if let Some(v) = self.left.as_ref() {
            struct_ser.serialize_field("left", v)?;
        }
        if !self.op.is_empty() {
            struct_ser.serialize_field("op", &self.op)?;
        }
        if let Some(v) = self.right.as_ref() {
            struct_ser.serialize_field("right", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatalogCompare {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "left",
            "op",
            "right",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Left,
            Op,
            Right,
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
                            "left" => Ok(GeneratedField::Left),
                            "op" => Ok(GeneratedField::Op),
                            "right" => Ok(GeneratedField::Right),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatalogCompare;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatalogCompare")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatalogCompare, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut left__ = None;
                let mut op__ = None;
                let mut right__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Left => {
                            if left__.is_some() {
                                return Err(serde::de::Error::duplicate_field("left"));
                            }
                            left__ = map_.next_value()?;
                        }
                        GeneratedField::Op => {
                            if op__.is_some() {
                                return Err(serde::de::Error::duplicate_field("op"));
                            }
                            op__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Right => {
                            if right__.is_some() {
                                return Err(serde::de::Error::duplicate_field("right"));
                            }
                            right__ = map_.next_value()?;
                        }
                    }
                }
                Ok(DatalogCompare {
                    left: left__,
                    op: op__.unwrap_or_default(),
                    right: right__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatalogCompare", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatalogExpr {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.op.is_empty() {
            len += 1;
        }
        if !self.func.is_empty() {
            len += 1;
        }
        if !self.args.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatalogExpr", len)?;
        if !self.op.is_empty() {
            struct_ser.serialize_field("op", &self.op)?;
        }
        if !self.func.is_empty() {
            struct_ser.serialize_field("func", &self.func)?;
        }
        if !self.args.is_empty() {
            struct_ser.serialize_field("args", &self.args)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatalogExpr {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "op",
            "func",
            "args",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Op,
            Func,
            Args,
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
                            "op" => Ok(GeneratedField::Op),
                            "func" => Ok(GeneratedField::Func),
                            "args" => Ok(GeneratedField::Args),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatalogExpr;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatalogExpr")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatalogExpr, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut op__ = None;
                let mut func__ = None;
                let mut args__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Op => {
                            if op__.is_some() {
                                return Err(serde::de::Error::duplicate_field("op"));
                            }
                            op__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Func => {
                            if func__.is_some() {
                                return Err(serde::de::Error::duplicate_field("func"));
                            }
                            func__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Args => {
                            if args__.is_some() {
                                return Err(serde::de::Error::duplicate_field("args"));
                            }
                            args__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(DatalogExpr {
                    op: op__.unwrap_or_default(),
                    func: func__.unwrap_or_default(),
                    args: args__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatalogExpr", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatalogLiteral {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.literal.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatalogLiteral", len)?;
        if let Some(v) = self.literal.as_ref() {
            match v {
                datalog_literal::Literal::Pos(v) => {
                    struct_ser.serialize_field("pos", v)?;
                }
                datalog_literal::Literal::Neg(v) => {
                    struct_ser.serialize_field("neg", v)?;
                }
                datalog_literal::Literal::Compare(v) => {
                    struct_ser.serialize_field("compare", v)?;
                }
                datalog_literal::Literal::Agg(v) => {
                    struct_ser.serialize_field("agg", v)?;
                }
                datalog_literal::Literal::Opt(v) => {
                    struct_ser.serialize_field("opt", v)?;
                }
            }
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatalogLiteral {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "pos",
            "neg",
            "compare",
            "agg",
            "opt",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Pos,
            Neg,
            Compare,
            Agg,
            Opt,
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
                            "pos" => Ok(GeneratedField::Pos),
                            "neg" => Ok(GeneratedField::Neg),
                            "compare" => Ok(GeneratedField::Compare),
                            "agg" => Ok(GeneratedField::Agg),
                            "opt" => Ok(GeneratedField::Opt),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatalogLiteral;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatalogLiteral")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatalogLiteral, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut literal__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Pos => {
                            if literal__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pos"));
                            }
                            literal__ = map_.next_value::<::std::option::Option<_>>()?.map(datalog_literal::Literal::Pos)
;
                        }
                        GeneratedField::Neg => {
                            if literal__.is_some() {
                                return Err(serde::de::Error::duplicate_field("neg"));
                            }
                            literal__ = map_.next_value::<::std::option::Option<_>>()?.map(datalog_literal::Literal::Neg)
;
                        }
                        GeneratedField::Compare => {
                            if literal__.is_some() {
                                return Err(serde::de::Error::duplicate_field("compare"));
                            }
                            literal__ = map_.next_value::<::std::option::Option<_>>()?.map(datalog_literal::Literal::Compare)
;
                        }
                        GeneratedField::Agg => {
                            if literal__.is_some() {
                                return Err(serde::de::Error::duplicate_field("agg"));
                            }
                            literal__ = map_.next_value::<::std::option::Option<_>>()?.map(datalog_literal::Literal::Agg)
;
                        }
                        GeneratedField::Opt => {
                            if literal__.is_some() {
                                return Err(serde::de::Error::duplicate_field("opt"));
                            }
                            literal__ = map_.next_value::<::std::option::Option<_>>()?.map(datalog_literal::Literal::Opt)
;
                        }
                    }
                }
                Ok(DatalogLiteral {
                    literal: literal__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatalogLiteral", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatalogOrder {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.term.is_some() {
            len += 1;
        }
        if self.desc {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatalogOrder", len)?;
        if let Some(v) = self.term.as_ref() {
            struct_ser.serialize_field("term", v)?;
        }
        if self.desc {
            struct_ser.serialize_field("desc", &self.desc)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatalogOrder {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "term",
            "desc",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Term,
            Desc,
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
                            "term" => Ok(GeneratedField::Term),
                            "desc" => Ok(GeneratedField::Desc),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatalogOrder;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatalogOrder")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatalogOrder, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut term__ = None;
                let mut desc__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Term => {
                            if term__.is_some() {
                                return Err(serde::de::Error::duplicate_field("term"));
                            }
                            term__ = map_.next_value()?;
                        }
                        GeneratedField::Desc => {
                            if desc__.is_some() {
                                return Err(serde::de::Error::duplicate_field("desc"));
                            }
                            desc__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(DatalogOrder {
                    term: term__,
                    desc: desc__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatalogOrder", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatalogQuery {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.rules.is_empty() {
            len += 1;
        }
        if self.goal.is_some() {
            len += 1;
        }
        if !self.select.is_empty() {
            len += 1;
        }
        if !self.having.is_empty() {
            len += 1;
        }
        if !self.order_by.is_empty() {
            len += 1;
        }
        if !self.after.is_empty() {
            len += 1;
        }
        if self.limit != 0 {
            len += 1;
        }
        if self.offset != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatalogQuery", len)?;
        if !self.rules.is_empty() {
            struct_ser.serialize_field("rules", &self.rules)?;
        }
        if let Some(v) = self.goal.as_ref() {
            struct_ser.serialize_field("goal", v)?;
        }
        if !self.select.is_empty() {
            struct_ser.serialize_field("select", &self.select)?;
        }
        if !self.having.is_empty() {
            struct_ser.serialize_field("having", &self.having)?;
        }
        if !self.order_by.is_empty() {
            struct_ser.serialize_field("orderBy", &self.order_by)?;
        }
        if !self.after.is_empty() {
            struct_ser.serialize_field("after", &self.after)?;
        }
        if self.limit != 0 {
            struct_ser.serialize_field("limit", &self.limit)?;
        }
        if self.offset != 0 {
            struct_ser.serialize_field("offset", &self.offset)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatalogQuery {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "rules",
            "goal",
            "select",
            "having",
            "order_by",
            "orderBy",
            "after",
            "limit",
            "offset",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Rules,
            Goal,
            Select,
            Having,
            OrderBy,
            After,
            Limit,
            Offset,
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
                            "rules" => Ok(GeneratedField::Rules),
                            "goal" => Ok(GeneratedField::Goal),
                            "select" => Ok(GeneratedField::Select),
                            "having" => Ok(GeneratedField::Having),
                            "orderBy" | "order_by" => Ok(GeneratedField::OrderBy),
                            "after" => Ok(GeneratedField::After),
                            "limit" => Ok(GeneratedField::Limit),
                            "offset" => Ok(GeneratedField::Offset),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatalogQuery;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatalogQuery")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatalogQuery, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut rules__ = None;
                let mut goal__ = None;
                let mut select__ = None;
                let mut having__ = None;
                let mut order_by__ = None;
                let mut after__ = None;
                let mut limit__ = None;
                let mut offset__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Rules => {
                            if rules__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rules"));
                            }
                            rules__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Goal => {
                            if goal__.is_some() {
                                return Err(serde::de::Error::duplicate_field("goal"));
                            }
                            goal__ = map_.next_value()?;
                        }
                        GeneratedField::Select => {
                            if select__.is_some() {
                                return Err(serde::de::Error::duplicate_field("select"));
                            }
                            select__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Having => {
                            if having__.is_some() {
                                return Err(serde::de::Error::duplicate_field("having"));
                            }
                            having__ = Some(map_.next_value()?);
                        }
                        GeneratedField::OrderBy => {
                            if order_by__.is_some() {
                                return Err(serde::de::Error::duplicate_field("orderBy"));
                            }
                            order_by__ = Some(map_.next_value()?);
                        }
                        GeneratedField::After => {
                            if after__.is_some() {
                                return Err(serde::de::Error::duplicate_field("after"));
                            }
                            after__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Limit => {
                            if limit__.is_some() {
                                return Err(serde::de::Error::duplicate_field("limit"));
                            }
                            limit__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Offset => {
                            if offset__.is_some() {
                                return Err(serde::de::Error::duplicate_field("offset"));
                            }
                            offset__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(DatalogQuery {
                    rules: rules__.unwrap_or_default(),
                    goal: goal__,
                    select: select__.unwrap_or_default(),
                    having: having__.unwrap_or_default(),
                    order_by: order_by__.unwrap_or_default(),
                    after: after__.unwrap_or_default(),
                    limit: limit__.unwrap_or_default(),
                    offset: offset__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatalogQuery", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatalogRule {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.head.is_some() {
            len += 1;
        }
        if self.body.is_some() {
            len += 1;
        }
        if self.hops != 0 {
            len += 1;
        }
        if !self.head_types.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatalogRule", len)?;
        if let Some(v) = self.head.as_ref() {
            struct_ser.serialize_field("head", v)?;
        }
        if let Some(v) = self.body.as_ref() {
            struct_ser.serialize_field("body", v)?;
        }
        if self.hops != 0 {
            struct_ser.serialize_field("hops", &self.hops)?;
        }
        if !self.head_types.is_empty() {
            struct_ser.serialize_field("headTypes", &self.head_types)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatalogRule {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "head",
            "body",
            "hops",
            "head_types",
            "headTypes",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Head,
            Body,
            Hops,
            HeadTypes,
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
                            "head" => Ok(GeneratedField::Head),
                            "body" => Ok(GeneratedField::Body),
                            "hops" => Ok(GeneratedField::Hops),
                            "headTypes" | "head_types" => Ok(GeneratedField::HeadTypes),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatalogRule;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatalogRule")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatalogRule, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut head__ = None;
                let mut body__ = None;
                let mut hops__ = None;
                let mut head_types__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Head => {
                            if head__.is_some() {
                                return Err(serde::de::Error::duplicate_field("head"));
                            }
                            head__ = map_.next_value()?;
                        }
                        GeneratedField::Body => {
                            if body__.is_some() {
                                return Err(serde::de::Error::duplicate_field("body"));
                            }
                            body__ = map_.next_value()?;
                        }
                        GeneratedField::Hops => {
                            if hops__.is_some() {
                                return Err(serde::de::Error::duplicate_field("hops"));
                            }
                            hops__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::HeadTypes => {
                            if head_types__.is_some() {
                                return Err(serde::de::Error::duplicate_field("headTypes"));
                            }
                            head_types__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(DatalogRule {
                    head: head__,
                    body: body__,
                    hops: hops__.unwrap_or_default(),
                    head_types: head_types__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatalogRule", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatalogTerm {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.term.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatalogTerm", len)?;
        if let Some(v) = self.term.as_ref() {
            match v {
                datalog_term::Term::Var(v) => {
                    struct_ser.serialize_field("var", v)?;
                }
                datalog_term::Term::Constant(v) => {
                    struct_ser.serialize_field("constant", v)?;
                }
                datalog_term::Term::Agg(v) => {
                    struct_ser.serialize_field("agg", v)?;
                }
                datalog_term::Term::Expr(v) => {
                    struct_ser.serialize_field("expr", v)?;
                }
            }
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatalogTerm {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "var",
            "constant",
            "agg",
            "expr",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Var,
            Constant,
            Agg,
            Expr,
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
                            "var" => Ok(GeneratedField::Var),
                            "constant" => Ok(GeneratedField::Constant),
                            "agg" => Ok(GeneratedField::Agg),
                            "expr" => Ok(GeneratedField::Expr),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatalogTerm;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatalogTerm")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatalogTerm, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut term__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Var => {
                            if term__.is_some() {
                                return Err(serde::de::Error::duplicate_field("var"));
                            }
                            term__ = map_.next_value::<::std::option::Option<_>>()?.map(datalog_term::Term::Var);
                        }
                        GeneratedField::Constant => {
                            if term__.is_some() {
                                return Err(serde::de::Error::duplicate_field("constant"));
                            }
                            term__ = map_.next_value::<::std::option::Option<_>>()?.map(datalog_term::Term::Constant)
;
                        }
                        GeneratedField::Agg => {
                            if term__.is_some() {
                                return Err(serde::de::Error::duplicate_field("agg"));
                            }
                            term__ = map_.next_value::<::std::option::Option<_>>()?.map(datalog_term::Term::Agg)
;
                        }
                        GeneratedField::Expr => {
                            if term__.is_some() {
                                return Err(serde::de::Error::duplicate_field("expr"));
                            }
                            term__ = map_.next_value::<::std::option::Option<_>>()?.map(datalog_term::Term::Expr)
;
                        }
                    }
                }
                Ok(DatalogTerm {
                    term: term__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatalogTerm", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatalogValue {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.s.is_empty() {
            len += 1;
        }
        if self.num.is_some() {
            len += 1;
        }
        if self.absent {
            len += 1;
        }
        if !self.base_unit.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatalogValue", len)?;
        if !self.s.is_empty() {
            struct_ser.serialize_field("s", &self.s)?;
        }
        if let Some(v) = self.num.as_ref() {
            struct_ser.serialize_field("num", v)?;
        }
        if self.absent {
            struct_ser.serialize_field("absent", &self.absent)?;
        }
        if !self.base_unit.is_empty() {
            struct_ser.serialize_field("baseUnit", &self.base_unit)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatalogValue {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "s",
            "num",
            "absent",
            "base_unit",
            "baseUnit",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            S,
            Num,
            Absent,
            BaseUnit,
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
                            "s" => Ok(GeneratedField::S),
                            "num" => Ok(GeneratedField::Num),
                            "absent" => Ok(GeneratedField::Absent),
                            "baseUnit" | "base_unit" => Ok(GeneratedField::BaseUnit),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatalogValue;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatalogValue")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatalogValue, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut s__ = None;
                let mut num__ = None;
                let mut absent__ = None;
                let mut base_unit__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::S => {
                            if s__.is_some() {
                                return Err(serde::de::Error::duplicate_field("s"));
                            }
                            s__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Num => {
                            if num__.is_some() {
                                return Err(serde::de::Error::duplicate_field("num"));
                            }
                            num__ = 
                                map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| x.0)
                            ;
                        }
                        GeneratedField::Absent => {
                            if absent__.is_some() {
                                return Err(serde::de::Error::duplicate_field("absent"));
                            }
                            absent__ = Some(map_.next_value()?);
                        }
                        GeneratedField::BaseUnit => {
                            if base_unit__.is_some() {
                                return Err(serde::de::Error::duplicate_field("baseUnit"));
                            }
                            base_unit__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(DatalogValue {
                    s: s__.unwrap_or_default(),
                    num: num__,
                    absent: absent__.unwrap_or_default(),
                    base_unit: base_unit__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatalogValue", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DatasheetCitation {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.doc.is_empty() {
            len += 1;
        }
        if !self.doc_ref.is_empty() {
            len += 1;
        }
        if self.page != 0 {
            len += 1;
        }
        if !self.section.is_empty() {
            len += 1;
        }
        if !self.method.is_empty() {
            len += 1;
        }
        if self.confidence != 0. {
            len += 1;
        }
        if !self.verification.is_empty() {
            len += 1;
        }
        if !self.verified_revision.is_empty() {
            len += 1;
        }
        if !self.corpus.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DatasheetCitation", len)?;
        if !self.doc.is_empty() {
            struct_ser.serialize_field("doc", &self.doc)?;
        }
        if !self.doc_ref.is_empty() {
            struct_ser.serialize_field("docRef", &self.doc_ref)?;
        }
        if self.page != 0 {
            struct_ser.serialize_field("page", &self.page)?;
        }
        if !self.section.is_empty() {
            struct_ser.serialize_field("section", &self.section)?;
        }
        if !self.method.is_empty() {
            struct_ser.serialize_field("method", &self.method)?;
        }
        if self.confidence != 0. {
            struct_ser.serialize_field("confidence", &self.confidence)?;
        }
        if !self.verification.is_empty() {
            struct_ser.serialize_field("verification", &self.verification)?;
        }
        if !self.verified_revision.is_empty() {
            struct_ser.serialize_field("verifiedRevision", &self.verified_revision)?;
        }
        if !self.corpus.is_empty() {
            struct_ser.serialize_field("corpus", &self.corpus)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DatasheetCitation {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "doc",
            "doc_ref",
            "docRef",
            "page",
            "section",
            "method",
            "confidence",
            "verification",
            "verified_revision",
            "verifiedRevision",
            "corpus",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Doc,
            DocRef,
            Page,
            Section,
            Method,
            Confidence,
            Verification,
            VerifiedRevision,
            Corpus,
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
                            "doc" => Ok(GeneratedField::Doc),
                            "docRef" | "doc_ref" => Ok(GeneratedField::DocRef),
                            "page" => Ok(GeneratedField::Page),
                            "section" => Ok(GeneratedField::Section),
                            "method" => Ok(GeneratedField::Method),
                            "confidence" => Ok(GeneratedField::Confidence),
                            "verification" => Ok(GeneratedField::Verification),
                            "verifiedRevision" | "verified_revision" => Ok(GeneratedField::VerifiedRevision),
                            "corpus" => Ok(GeneratedField::Corpus),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DatasheetCitation;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DatasheetCitation")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DatasheetCitation, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut doc__ = None;
                let mut doc_ref__ = None;
                let mut page__ = None;
                let mut section__ = None;
                let mut method__ = None;
                let mut confidence__ = None;
                let mut verification__ = None;
                let mut verified_revision__ = None;
                let mut corpus__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Doc => {
                            if doc__.is_some() {
                                return Err(serde::de::Error::duplicate_field("doc"));
                            }
                            doc__ = Some(map_.next_value()?);
                        }
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
                        GeneratedField::Section => {
                            if section__.is_some() {
                                return Err(serde::de::Error::duplicate_field("section"));
                            }
                            section__ = Some(map_.next_value()?);
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
                        GeneratedField::Verification => {
                            if verification__.is_some() {
                                return Err(serde::de::Error::duplicate_field("verification"));
                            }
                            verification__ = Some(map_.next_value()?);
                        }
                        GeneratedField::VerifiedRevision => {
                            if verified_revision__.is_some() {
                                return Err(serde::de::Error::duplicate_field("verifiedRevision"));
                            }
                            verified_revision__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Corpus => {
                            if corpus__.is_some() {
                                return Err(serde::de::Error::duplicate_field("corpus"));
                            }
                            corpus__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(DatasheetCitation {
                    doc: doc__.unwrap_or_default(),
                    doc_ref: doc_ref__.unwrap_or_default(),
                    page: page__.unwrap_or_default(),
                    section: section__.unwrap_or_default(),
                    method: method__.unwrap_or_default(),
                    confidence: confidence__.unwrap_or_default(),
                    verification: verification__.unwrap_or_default(),
                    verified_revision: verified_revision__.unwrap_or_default(),
                    corpus: corpus__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DatasheetCitation", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for DesignRef {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.source.is_empty() {
            len += 1;
        }
        if !self.content_hash.is_empty() {
            len += 1;
        }
        if !self.unexpanded_hierarchy.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.DesignRef", len)?;
        if !self.source.is_empty() {
            struct_ser.serialize_field("source", &self.source)?;
        }
        if !self.content_hash.is_empty() {
            struct_ser.serialize_field("contentHash", &self.content_hash)?;
        }
        if !self.unexpanded_hierarchy.is_empty() {
            struct_ser.serialize_field("unexpandedHierarchy", &self.unexpanded_hierarchy)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for DesignRef {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "source",
            "content_hash",
            "contentHash",
            "unexpanded_hierarchy",
            "unexpandedHierarchy",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Source,
            ContentHash,
            UnexpandedHierarchy,
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
                            "source" => Ok(GeneratedField::Source),
                            "contentHash" | "content_hash" => Ok(GeneratedField::ContentHash),
                            "unexpandedHierarchy" | "unexpanded_hierarchy" => Ok(GeneratedField::UnexpandedHierarchy),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = DesignRef;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.DesignRef")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<DesignRef, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut source__ = None;
                let mut content_hash__ = None;
                let mut unexpanded_hierarchy__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Source => {
                            if source__.is_some() {
                                return Err(serde::de::Error::duplicate_field("source"));
                            }
                            source__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ContentHash => {
                            if content_hash__.is_some() {
                                return Err(serde::de::Error::duplicate_field("contentHash"));
                            }
                            content_hash__ = Some(map_.next_value()?);
                        }
                        GeneratedField::UnexpandedHierarchy => {
                            if unexpanded_hierarchy__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unexpandedHierarchy"));
                            }
                            unexpanded_hierarchy__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(DesignRef {
                    source: source__.unwrap_or_default(),
                    content_hash: content_hash__.unwrap_or_default(),
                    unexpanded_hierarchy: unexpanded_hierarchy__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.DesignRef", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Finding {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.rule.is_empty() {
            len += 1;
        }
        if !self.severity.is_empty() {
            len += 1;
        }
        if self.subject.is_some() {
            len += 1;
        }
        if !self.message.is_empty() {
            len += 1;
        }
        if self.inconclusive {
            len += 1;
        }
        if self.provenance.is_some() {
            len += 1;
        }
        if !self.sheets.is_empty() {
            len += 1;
        }
        if self.locate_reason != 0 {
            len += 1;
        }
        if !self.datasheets.is_empty() {
            len += 1;
        }
        if !self.context.is_empty() {
            len += 1;
        }
        if !self.verdict_ids.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.Finding", len)?;
        if !self.rule.is_empty() {
            struct_ser.serialize_field("rule", &self.rule)?;
        }
        if !self.severity.is_empty() {
            struct_ser.serialize_field("severity", &self.severity)?;
        }
        if let Some(v) = self.subject.as_ref() {
            struct_ser.serialize_field("subject", v)?;
        }
        if !self.message.is_empty() {
            struct_ser.serialize_field("message", &self.message)?;
        }
        if self.inconclusive {
            struct_ser.serialize_field("inconclusive", &self.inconclusive)?;
        }
        if let Some(v) = self.provenance.as_ref() {
            struct_ser.serialize_field("provenance", v)?;
        }
        if !self.sheets.is_empty() {
            struct_ser.serialize_field("sheets", &self.sheets)?;
        }
        if self.locate_reason != 0 {
            let v = LocateReason::try_from(self.locate_reason)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.locate_reason)))?;
            struct_ser.serialize_field("locateReason", &v)?;
        }
        if !self.datasheets.is_empty() {
            struct_ser.serialize_field("datasheets", &self.datasheets)?;
        }
        if !self.context.is_empty() {
            struct_ser.serialize_field("context", &self.context)?;
        }
        if !self.verdict_ids.is_empty() {
            struct_ser.serialize_field("verdictIds", &self.verdict_ids)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Finding {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "rule",
            "severity",
            "subject",
            "message",
            "inconclusive",
            "provenance",
            "sheets",
            "locate_reason",
            "locateReason",
            "datasheets",
            "context",
            "verdict_ids",
            "verdictIds",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Rule,
            Severity,
            Subject,
            Message,
            Inconclusive,
            Provenance,
            Sheets,
            LocateReason,
            Datasheets,
            Context,
            VerdictIds,
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
                            "rule" => Ok(GeneratedField::Rule),
                            "severity" => Ok(GeneratedField::Severity),
                            "subject" => Ok(GeneratedField::Subject),
                            "message" => Ok(GeneratedField::Message),
                            "inconclusive" => Ok(GeneratedField::Inconclusive),
                            "provenance" => Ok(GeneratedField::Provenance),
                            "sheets" => Ok(GeneratedField::Sheets),
                            "locateReason" | "locate_reason" => Ok(GeneratedField::LocateReason),
                            "datasheets" => Ok(GeneratedField::Datasheets),
                            "context" => Ok(GeneratedField::Context),
                            "verdictIds" | "verdict_ids" => Ok(GeneratedField::VerdictIds),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Finding;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.Finding")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Finding, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut rule__ = None;
                let mut severity__ = None;
                let mut subject__ = None;
                let mut message__ = None;
                let mut inconclusive__ = None;
                let mut provenance__ = None;
                let mut sheets__ = None;
                let mut locate_reason__ = None;
                let mut datasheets__ = None;
                let mut context__ = None;
                let mut verdict_ids__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Rule => {
                            if rule__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rule"));
                            }
                            rule__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Severity => {
                            if severity__.is_some() {
                                return Err(serde::de::Error::duplicate_field("severity"));
                            }
                            severity__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Subject => {
                            if subject__.is_some() {
                                return Err(serde::de::Error::duplicate_field("subject"));
                            }
                            subject__ = map_.next_value()?;
                        }
                        GeneratedField::Message => {
                            if message__.is_some() {
                                return Err(serde::de::Error::duplicate_field("message"));
                            }
                            message__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Inconclusive => {
                            if inconclusive__.is_some() {
                                return Err(serde::de::Error::duplicate_field("inconclusive"));
                            }
                            inconclusive__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Provenance => {
                            if provenance__.is_some() {
                                return Err(serde::de::Error::duplicate_field("provenance"));
                            }
                            provenance__ = map_.next_value()?;
                        }
                        GeneratedField::Sheets => {
                            if sheets__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sheets"));
                            }
                            sheets__ = Some(map_.next_value()?);
                        }
                        GeneratedField::LocateReason => {
                            if locate_reason__.is_some() {
                                return Err(serde::de::Error::duplicate_field("locateReason"));
                            }
                            locate_reason__ = Some(map_.next_value::<LocateReason>()? as i32);
                        }
                        GeneratedField::Datasheets => {
                            if datasheets__.is_some() {
                                return Err(serde::de::Error::duplicate_field("datasheets"));
                            }
                            datasheets__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Context => {
                            if context__.is_some() {
                                return Err(serde::de::Error::duplicate_field("context"));
                            }
                            context__ = Some(map_.next_value()?);
                        }
                        GeneratedField::VerdictIds => {
                            if verdict_ids__.is_some() {
                                return Err(serde::de::Error::duplicate_field("verdictIds"));
                            }
                            verdict_ids__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Finding {
                    rule: rule__.unwrap_or_default(),
                    severity: severity__.unwrap_or_default(),
                    subject: subject__,
                    message: message__.unwrap_or_default(),
                    inconclusive: inconclusive__.unwrap_or_default(),
                    provenance: provenance__,
                    sheets: sheets__.unwrap_or_default(),
                    locate_reason: locate_reason__.unwrap_or_default(),
                    datasheets: datasheets__.unwrap_or_default(),
                    context: context__.unwrap_or_default(),
                    verdict_ids: verdict_ids__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.Finding", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ImportSummary {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.findings != 0 {
            len += 1;
        }
        if self.joined != 0 {
            len += 1;
        }
        if !self.unjoined.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ImportSummary", len)?;
        if self.findings != 0 {
            struct_ser.serialize_field("findings", &self.findings)?;
        }
        if self.joined != 0 {
            struct_ser.serialize_field("joined", &self.joined)?;
        }
        if !self.unjoined.is_empty() {
            struct_ser.serialize_field("unjoined", &self.unjoined)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ImportSummary {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "findings",
            "joined",
            "unjoined",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Findings,
            Joined,
            Unjoined,
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
                            "findings" => Ok(GeneratedField::Findings),
                            "joined" => Ok(GeneratedField::Joined),
                            "unjoined" => Ok(GeneratedField::Unjoined),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ImportSummary;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ImportSummary")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ImportSummary, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut findings__ = None;
                let mut joined__ = None;
                let mut unjoined__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Findings => {
                            if findings__.is_some() {
                                return Err(serde::de::Error::duplicate_field("findings"));
                            }
                            findings__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Joined => {
                            if joined__.is_some() {
                                return Err(serde::de::Error::duplicate_field("joined"));
                            }
                            joined__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Unjoined => {
                            if unjoined__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unjoined"));
                            }
                            unjoined__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ImportSummary {
                    findings: findings__.unwrap_or_default(),
                    joined: joined__.unwrap_or_default(),
                    unjoined: unjoined__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ImportSummary", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ItemBinding {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.rule.is_empty() {
            len += 1;
        }
        if !self.tag.is_empty() {
            len += 1;
        }
        if !self.profile.is_empty() {
            len += 1;
        }
        if self.query.is_some() {
            len += 1;
        }
        if self.present.is_some() {
            len += 1;
        }
        if self.scope.is_some() {
            len += 1;
        }
        if !self.requirement.is_empty() {
            len += 1;
        }
        if !self.applies_to_class.is_empty() {
            len += 1;
        }
        if self.external.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ItemBinding", len)?;
        if !self.rule.is_empty() {
            struct_ser.serialize_field("rule", &self.rule)?;
        }
        if !self.tag.is_empty() {
            struct_ser.serialize_field("tag", &self.tag)?;
        }
        if !self.profile.is_empty() {
            struct_ser.serialize_field("profile", &self.profile)?;
        }
        if let Some(v) = self.query.as_ref() {
            struct_ser.serialize_field("query", v)?;
        }
        if let Some(v) = self.present.as_ref() {
            struct_ser.serialize_field("present", v)?;
        }
        if let Some(v) = self.scope.as_ref() {
            struct_ser.serialize_field("scope", v)?;
        }
        if !self.requirement.is_empty() {
            struct_ser.serialize_field("requirement", &self.requirement)?;
        }
        if !self.applies_to_class.is_empty() {
            struct_ser.serialize_field("appliesToClass", &self.applies_to_class)?;
        }
        if let Some(v) = self.external.as_ref() {
            struct_ser.serialize_field("external", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ItemBinding {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "rule",
            "tag",
            "profile",
            "query",
            "present",
            "scope",
            "requirement",
            "applies_to_class",
            "appliesToClass",
            "external",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Rule,
            Tag,
            Profile,
            Query,
            Present,
            Scope,
            Requirement,
            AppliesToClass,
            External,
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
                            "rule" => Ok(GeneratedField::Rule),
                            "tag" => Ok(GeneratedField::Tag),
                            "profile" => Ok(GeneratedField::Profile),
                            "query" => Ok(GeneratedField::Query),
                            "present" => Ok(GeneratedField::Present),
                            "scope" => Ok(GeneratedField::Scope),
                            "requirement" => Ok(GeneratedField::Requirement),
                            "appliesToClass" | "applies_to_class" => Ok(GeneratedField::AppliesToClass),
                            "external" => Ok(GeneratedField::External),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ItemBinding;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ItemBinding")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ItemBinding, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut rule__ = None;
                let mut tag__ = None;
                let mut profile__ = None;
                let mut query__ = None;
                let mut present__ = None;
                let mut scope__ = None;
                let mut requirement__ = None;
                let mut applies_to_class__ = None;
                let mut external__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Rule => {
                            if rule__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rule"));
                            }
                            rule__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Tag => {
                            if tag__.is_some() {
                                return Err(serde::de::Error::duplicate_field("tag"));
                            }
                            tag__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Profile => {
                            if profile__.is_some() {
                                return Err(serde::de::Error::duplicate_field("profile"));
                            }
                            profile__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Query => {
                            if query__.is_some() {
                                return Err(serde::de::Error::duplicate_field("query"));
                            }
                            query__ = map_.next_value()?;
                        }
                        GeneratedField::Present => {
                            if present__.is_some() {
                                return Err(serde::de::Error::duplicate_field("present"));
                            }
                            present__ = map_.next_value()?;
                        }
                        GeneratedField::Scope => {
                            if scope__.is_some() {
                                return Err(serde::de::Error::duplicate_field("scope"));
                            }
                            scope__ = map_.next_value()?;
                        }
                        GeneratedField::Requirement => {
                            if requirement__.is_some() {
                                return Err(serde::de::Error::duplicate_field("requirement"));
                            }
                            requirement__ = Some(map_.next_value()?);
                        }
                        GeneratedField::AppliesToClass => {
                            if applies_to_class__.is_some() {
                                return Err(serde::de::Error::duplicate_field("appliesToClass"));
                            }
                            applies_to_class__ = Some(map_.next_value()?);
                        }
                        GeneratedField::External => {
                            if external__.is_some() {
                                return Err(serde::de::Error::duplicate_field("external"));
                            }
                            external__ = map_.next_value()?;
                        }
                    }
                }
                Ok(ItemBinding {
                    rule: rule__.unwrap_or_default(),
                    tag: tag__.unwrap_or_default(),
                    profile: profile__.unwrap_or_default(),
                    query: query__,
                    present: present__,
                    scope: scope__,
                    requirement: requirement__.unwrap_or_default(),
                    applies_to_class: applies_to_class__.unwrap_or_default(),
                    external: external__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ItemBinding", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for LocateReason {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "LOCATE_REASON_UNSPECIFIED",
            Self::VirtualSymbol => "LOCATE_REASON_VIRTUAL_SYMBOL",
            Self::PowerRailNoWire => "LOCATE_REASON_POWER_RAIL_NO_WIRE",
            Self::NotInDesign => "LOCATE_REASON_NOT_IN_DESIGN",
            Self::NoGeometry => "LOCATE_REASON_NO_GEOMETRY",
            Self::BusNotDrawn => "LOCATE_REASON_BUS_NOT_DRAWN",
            Self::SchematicUnreadable => "LOCATE_REASON_SCHEMATIC_UNREADABLE",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for LocateReason {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "LOCATE_REASON_UNSPECIFIED",
            "LOCATE_REASON_VIRTUAL_SYMBOL",
            "LOCATE_REASON_POWER_RAIL_NO_WIRE",
            "LOCATE_REASON_NOT_IN_DESIGN",
            "LOCATE_REASON_NO_GEOMETRY",
            "LOCATE_REASON_BUS_NOT_DRAWN",
            "LOCATE_REASON_SCHEMATIC_UNREADABLE",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = LocateReason;

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
                    "LOCATE_REASON_UNSPECIFIED" => Ok(LocateReason::Unspecified),
                    "LOCATE_REASON_VIRTUAL_SYMBOL" => Ok(LocateReason::VirtualSymbol),
                    "LOCATE_REASON_POWER_RAIL_NO_WIRE" => Ok(LocateReason::PowerRailNoWire),
                    "LOCATE_REASON_NOT_IN_DESIGN" => Ok(LocateReason::NotInDesign),
                    "LOCATE_REASON_NO_GEOMETRY" => Ok(LocateReason::NoGeometry),
                    "LOCATE_REASON_BUS_NOT_DRAWN" => Ok(LocateReason::BusNotDrawn),
                    "LOCATE_REASON_SCHEMATIC_UNREADABLE" => Ok(LocateReason::SchematicUnreadable),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for ManifestArea {
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
        if !self.items.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ManifestArea", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.items.is_empty() {
            struct_ser.serialize_field("items", &self.items)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ManifestArea {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "items",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Items,
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
                            "items" => Ok(GeneratedField::Items),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ManifestArea;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ManifestArea")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ManifestArea, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut items__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Items => {
                            if items__.is_some() {
                                return Err(serde::de::Error::duplicate_field("items"));
                            }
                            items__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ManifestArea {
                    name: name__.unwrap_or_default(),
                    items: items__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ManifestArea", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ManifestExternal {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.producer.is_empty() {
            len += 1;
        }
        if !self.rule.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ManifestExternal", len)?;
        if !self.producer.is_empty() {
            struct_ser.serialize_field("producer", &self.producer)?;
        }
        if !self.rule.is_empty() {
            struct_ser.serialize_field("rule", &self.rule)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ManifestExternal {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "producer",
            "rule",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Producer,
            Rule,
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
                            "producer" => Ok(GeneratedField::Producer),
                            "rule" => Ok(GeneratedField::Rule),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ManifestExternal;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ManifestExternal")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ManifestExternal, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut producer__ = None;
                let mut rule__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Producer => {
                            if producer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("producer"));
                            }
                            producer__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Rule => {
                            if rule__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rule"));
                            }
                            rule__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ManifestExternal {
                    producer: producer__.unwrap_or_default(),
                    rule: rule__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ManifestExternal", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ManifestItem {
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
        if !self.description.is_empty() {
            len += 1;
        }
        if !self.note.is_empty() {
            len += 1;
        }
        if self.binding.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ManifestItem", len)?;
        if !self.id.is_empty() {
            struct_ser.serialize_field("id", &self.id)?;
        }
        if !self.title.is_empty() {
            struct_ser.serialize_field("title", &self.title)?;
        }
        if !self.description.is_empty() {
            struct_ser.serialize_field("description", &self.description)?;
        }
        if !self.note.is_empty() {
            struct_ser.serialize_field("note", &self.note)?;
        }
        if let Some(v) = self.binding.as_ref() {
            struct_ser.serialize_field("binding", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ManifestItem {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "id",
            "title",
            "description",
            "note",
            "binding",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Id,
            Title,
            Description,
            Note,
            Binding,
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
                            "description" => Ok(GeneratedField::Description),
                            "note" => Ok(GeneratedField::Note),
                            "binding" => Ok(GeneratedField::Binding),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ManifestItem;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ManifestItem")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ManifestItem, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut id__ = None;
                let mut title__ = None;
                let mut description__ = None;
                let mut note__ = None;
                let mut binding__ = None;
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
                        GeneratedField::Description => {
                            if description__.is_some() {
                                return Err(serde::de::Error::duplicate_field("description"));
                            }
                            description__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Note => {
                            if note__.is_some() {
                                return Err(serde::de::Error::duplicate_field("note"));
                            }
                            note__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Binding => {
                            if binding__.is_some() {
                                return Err(serde::de::Error::duplicate_field("binding"));
                            }
                            binding__ = map_.next_value()?;
                        }
                    }
                }
                Ok(ManifestItem {
                    id: id__.unwrap_or_default(),
                    title: title__.unwrap_or_default(),
                    description: description__.unwrap_or_default(),
                    note: note__.unwrap_or_default(),
                    binding: binding__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ManifestItem", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ManifestPresent {
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
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ManifestPresent", len)?;
        if !self.class.is_empty() {
            struct_ser.serialize_field("class", &self.class)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ManifestPresent {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "class",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Class,
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
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ManifestPresent;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ManifestPresent")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ManifestPresent, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut class__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Class => {
                            if class__.is_some() {
                                return Err(serde::de::Error::duplicate_field("class"));
                            }
                            class__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ManifestPresent {
                    class: class__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ManifestPresent", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ManifestQuery {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.r#match.is_empty() {
            len += 1;
        }
        if !self.subject.is_empty() {
            len += 1;
        }
        if !self.kind.is_empty() {
            len += 1;
        }
        if !self.message.is_empty() {
            len += 1;
        }
        if !self.severity.is_empty() {
            len += 1;
        }
        if !self.param_symbol.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ManifestQuery", len)?;
        if !self.r#match.is_empty() {
            struct_ser.serialize_field("match", &self.r#match)?;
        }
        if !self.subject.is_empty() {
            struct_ser.serialize_field("subject", &self.subject)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if !self.message.is_empty() {
            struct_ser.serialize_field("message", &self.message)?;
        }
        if !self.severity.is_empty() {
            struct_ser.serialize_field("severity", &self.severity)?;
        }
        if !self.param_symbol.is_empty() {
            struct_ser.serialize_field("paramSymbol", &self.param_symbol)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ManifestQuery {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "match",
            "subject",
            "kind",
            "message",
            "severity",
            "param_symbol",
            "paramSymbol",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Match,
            Subject,
            Kind,
            Message,
            Severity,
            ParamSymbol,
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
                            "match" => Ok(GeneratedField::Match),
                            "subject" => Ok(GeneratedField::Subject),
                            "kind" => Ok(GeneratedField::Kind),
                            "message" => Ok(GeneratedField::Message),
                            "severity" => Ok(GeneratedField::Severity),
                            "paramSymbol" | "param_symbol" => Ok(GeneratedField::ParamSymbol),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ManifestQuery;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ManifestQuery")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ManifestQuery, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut r#match__ = None;
                let mut subject__ = None;
                let mut kind__ = None;
                let mut message__ = None;
                let mut severity__ = None;
                let mut param_symbol__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Match => {
                            if r#match__.is_some() {
                                return Err(serde::de::Error::duplicate_field("match"));
                            }
                            r#match__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Subject => {
                            if subject__.is_some() {
                                return Err(serde::de::Error::duplicate_field("subject"));
                            }
                            subject__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Message => {
                            if message__.is_some() {
                                return Err(serde::de::Error::duplicate_field("message"));
                            }
                            message__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Severity => {
                            if severity__.is_some() {
                                return Err(serde::de::Error::duplicate_field("severity"));
                            }
                            severity__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ParamSymbol => {
                            if param_symbol__.is_some() {
                                return Err(serde::de::Error::duplicate_field("paramSymbol"));
                            }
                            param_symbol__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ManifestQuery {
                    r#match: r#match__.unwrap_or_default(),
                    subject: subject__.unwrap_or_default(),
                    kind: kind__.unwrap_or_default(),
                    message: message__.unwrap_or_default(),
                    severity: severity__.unwrap_or_default(),
                    param_symbol: param_symbol__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ManifestQuery", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ManifestScope {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.profiles.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ManifestScope", len)?;
        if !self.profiles.is_empty() {
            struct_ser.serialize_field("profiles", &self.profiles)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ManifestScope {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "profiles",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Profiles,
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
                            "profiles" => Ok(GeneratedField::Profiles),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ManifestScope;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ManifestScope")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ManifestScope, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut profiles__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Profiles => {
                            if profiles__.is_some() {
                                return Err(serde::de::Error::duplicate_field("profiles"));
                            }
                            profiles__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ManifestScope {
                    profiles: profiles__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ManifestScope", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Outcome {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let variant = match self {
            Self::Unspecified => "OUTCOME_UNSPECIFIED",
            Self::Pass => "OUTCOME_PASS",
            Self::Fail => "OUTCOME_FAIL",
            Self::NoLimit => "OUTCOME_NO_LIMIT",
            Self::NotConsidered => "OUTCOME_NOT_CONSIDERED",
            Self::Inconclusive => "OUTCOME_INCONCLUSIVE",
        };
        serializer.serialize_str(variant)
    }
}
impl<'de> serde::Deserialize<'de> for Outcome {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "OUTCOME_UNSPECIFIED",
            "OUTCOME_PASS",
            "OUTCOME_FAIL",
            "OUTCOME_NO_LIMIT",
            "OUTCOME_NOT_CONSIDERED",
            "OUTCOME_INCONCLUSIVE",
        ];

        struct GeneratedVisitor;

        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Outcome;

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
                    "OUTCOME_UNSPECIFIED" => Ok(Outcome::Unspecified),
                    "OUTCOME_PASS" => Ok(Outcome::Pass),
                    "OUTCOME_FAIL" => Ok(Outcome::Fail),
                    "OUTCOME_NO_LIMIT" => Ok(Outcome::NoLimit),
                    "OUTCOME_NOT_CONSIDERED" => Ok(Outcome::NotConsidered),
                    "OUTCOME_INCONCLUSIVE" => Ok(Outcome::Inconclusive),
                    _ => Err(serde::de::Error::unknown_variant(value, FIELDS)),
                }
            }
        }
        deserializer.deserialize_any(GeneratedVisitor)
    }
}
impl serde::Serialize for ProfileDef {
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
        if !self.signals.is_empty() {
            len += 1;
        }
        if !self.requirements.is_empty() {
            len += 1;
        }
        if self.host.is_some() {
            len += 1;
        }
        if !self.instance.is_empty() {
            len += 1;
        }
        if !self.excludes.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ProfileDef", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.signals.is_empty() {
            struct_ser.serialize_field("signals", &self.signals)?;
        }
        if !self.requirements.is_empty() {
            struct_ser.serialize_field("requirements", &self.requirements)?;
        }
        if let Some(v) = self.host.as_ref() {
            struct_ser.serialize_field("host", v)?;
        }
        if !self.instance.is_empty() {
            struct_ser.serialize_field("instance", &self.instance)?;
        }
        if !self.excludes.is_empty() {
            struct_ser.serialize_field("excludes", &self.excludes)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ProfileDef {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "signals",
            "requirements",
            "host",
            "instance",
            "excludes",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Signals,
            Requirements,
            Host,
            Instance,
            Excludes,
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
                            "signals" => Ok(GeneratedField::Signals),
                            "requirements" => Ok(GeneratedField::Requirements),
                            "host" => Ok(GeneratedField::Host),
                            "instance" => Ok(GeneratedField::Instance),
                            "excludes" => Ok(GeneratedField::Excludes),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ProfileDef;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ProfileDef")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ProfileDef, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut signals__ = None;
                let mut requirements__ = None;
                let mut host__ = None;
                let mut instance__ = None;
                let mut excludes__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Signals => {
                            if signals__.is_some() {
                                return Err(serde::de::Error::duplicate_field("signals"));
                            }
                            signals__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Requirements => {
                            if requirements__.is_some() {
                                return Err(serde::de::Error::duplicate_field("requirements"));
                            }
                            requirements__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Host => {
                            if host__.is_some() {
                                return Err(serde::de::Error::duplicate_field("host"));
                            }
                            host__ = map_.next_value()?;
                        }
                        GeneratedField::Instance => {
                            if instance__.is_some() {
                                return Err(serde::de::Error::duplicate_field("instance"));
                            }
                            instance__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Excludes => {
                            if excludes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("excludes"));
                            }
                            excludes__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ProfileDef {
                    name: name__.unwrap_or_default(),
                    signals: signals__.unwrap_or_default(),
                    requirements: requirements__.unwrap_or_default(),
                    host: host__,
                    instance: instance__.unwrap_or_default(),
                    excludes: excludes__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ProfileDef", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ProfileHost {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.attr.is_empty() {
            len += 1;
        }
        if !self.value.is_empty() {
            len += 1;
        }
        if !self.class.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ProfileHost", len)?;
        if !self.attr.is_empty() {
            struct_ser.serialize_field("attr", &self.attr)?;
        }
        if !self.value.is_empty() {
            struct_ser.serialize_field("value", &self.value)?;
        }
        if !self.class.is_empty() {
            struct_ser.serialize_field("class", &self.class)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ProfileHost {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "attr",
            "value",
            "class",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Attr,
            Value,
            Class,
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
                            "attr" => Ok(GeneratedField::Attr),
                            "value" => Ok(GeneratedField::Value),
                            "class" => Ok(GeneratedField::Class),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ProfileHost;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ProfileHost")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ProfileHost, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut attr__ = None;
                let mut value__ = None;
                let mut class__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Attr => {
                            if attr__.is_some() {
                                return Err(serde::de::Error::duplicate_field("attr"));
                            }
                            attr__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Value => {
                            if value__.is_some() {
                                return Err(serde::de::Error::duplicate_field("value"));
                            }
                            value__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Class => {
                            if class__.is_some() {
                                return Err(serde::de::Error::duplicate_field("class"));
                            }
                            class__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ProfileHost {
                    attr: attr__.unwrap_or_default(),
                    value: value__.unwrap_or_default(),
                    class: class__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ProfileHost", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ProfileNamingMap {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.r#override.is_empty() {
            len += 1;
        }
        if !self.suffixes.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ProfileNamingMap", len)?;
        if !self.r#override.is_empty() {
            struct_ser.serialize_field("override", &self.r#override)?;
        }
        if !self.suffixes.is_empty() {
            struct_ser.serialize_field("suffixes", &self.suffixes)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ProfileNamingMap {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "override",
            "suffixes",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Override,
            Suffixes,
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
                            "override" => Ok(GeneratedField::Override),
                            "suffixes" => Ok(GeneratedField::Suffixes),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ProfileNamingMap;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ProfileNamingMap")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ProfileNamingMap, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut r#override__ = None;
                let mut suffixes__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Override => {
                            if r#override__.is_some() {
                                return Err(serde::de::Error::duplicate_field("override"));
                            }
                            r#override__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Suffixes => {
                            if suffixes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("suffixes"));
                            }
                            suffixes__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                    }
                }
                Ok(ProfileNamingMap {
                    r#override: r#override__.unwrap_or_default(),
                    suffixes: suffixes__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ProfileNamingMap", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ProfileRequirement {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.r#type.is_empty() {
            len += 1;
        }
        if !self.params.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ProfileRequirement", len)?;
        if !self.r#type.is_empty() {
            struct_ser.serialize_field("type", &self.r#type)?;
        }
        if !self.params.is_empty() {
            struct_ser.serialize_field("params", &self.params)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ProfileRequirement {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "type",
            "params",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Type,
            Params,
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
                            "type" => Ok(GeneratedField::Type),
                            "params" => Ok(GeneratedField::Params),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ProfileRequirement;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ProfileRequirement")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ProfileRequirement, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut r#type__ = None;
                let mut params__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Type => {
                            if r#type__.is_some() {
                                return Err(serde::de::Error::duplicate_field("type"));
                            }
                            r#type__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Params => {
                            if params__.is_some() {
                                return Err(serde::de::Error::duplicate_field("params"));
                            }
                            params__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                    }
                }
                Ok(ProfileRequirement {
                    r#type: r#type__.unwrap_or_default(),
                    params: params__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ProfileRequirement", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ProfileSignal {
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
        if !self.prefix.is_empty() {
            len += 1;
        }
        if !self.suffix.is_empty() {
            len += 1;
        }
        if !self.glob.is_empty() {
            len += 1;
        }
        if !self.regex.is_empty() {
            len += 1;
        }
        if self.pullup {
            len += 1;
        }
        if self.anchor {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ProfileSignal", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.prefix.is_empty() {
            struct_ser.serialize_field("prefix", &self.prefix)?;
        }
        if !self.suffix.is_empty() {
            struct_ser.serialize_field("suffix", &self.suffix)?;
        }
        if !self.glob.is_empty() {
            struct_ser.serialize_field("glob", &self.glob)?;
        }
        if !self.regex.is_empty() {
            struct_ser.serialize_field("regex", &self.regex)?;
        }
        if self.pullup {
            struct_ser.serialize_field("pullup", &self.pullup)?;
        }
        if self.anchor {
            struct_ser.serialize_field("anchor", &self.anchor)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ProfileSignal {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "prefix",
            "suffix",
            "glob",
            "regex",
            "pullup",
            "anchor",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Prefix,
            Suffix,
            Glob,
            Regex,
            Pullup,
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
                            "name" => Ok(GeneratedField::Name),
                            "prefix" => Ok(GeneratedField::Prefix),
                            "suffix" => Ok(GeneratedField::Suffix),
                            "glob" => Ok(GeneratedField::Glob),
                            "regex" => Ok(GeneratedField::Regex),
                            "pullup" => Ok(GeneratedField::Pullup),
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
            type Value = ProfileSignal;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ProfileSignal")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ProfileSignal, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut prefix__ = None;
                let mut suffix__ = None;
                let mut glob__ = None;
                let mut regex__ = None;
                let mut pullup__ = None;
                let mut anchor__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Prefix => {
                            if prefix__.is_some() {
                                return Err(serde::de::Error::duplicate_field("prefix"));
                            }
                            prefix__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Suffix => {
                            if suffix__.is_some() {
                                return Err(serde::de::Error::duplicate_field("suffix"));
                            }
                            suffix__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Glob => {
                            if glob__.is_some() {
                                return Err(serde::de::Error::duplicate_field("glob"));
                            }
                            glob__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Regex => {
                            if regex__.is_some() {
                                return Err(serde::de::Error::duplicate_field("regex"));
                            }
                            regex__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Pullup => {
                            if pullup__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pullup"));
                            }
                            pullup__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Anchor => {
                            if anchor__.is_some() {
                                return Err(serde::de::Error::duplicate_field("anchor"));
                            }
                            anchor__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ProfileSignal {
                    name: name__.unwrap_or_default(),
                    prefix: prefix__.unwrap_or_default(),
                    suffix: suffix__.unwrap_or_default(),
                    glob: glob__.unwrap_or_default(),
                    regex: regex__.unwrap_or_default(),
                    pullup: pullup__.unwrap_or_default(),
                    anchor: anchor__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ProfileSignal", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for QueryRule {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.meta.is_some() {
            len += 1;
        }
        if self.query.is_some() {
            len += 1;
        }
        if !self.kind.is_empty() {
            len += 1;
        }
        if !self.subject_var.is_empty() {
            len += 1;
        }
        if !self.pin_var.is_empty() {
            len += 1;
        }
        if !self.message.is_empty() {
            len += 1;
        }
        if !self.param_symbol.is_empty() {
            len += 1;
        }
        if !self.context_vars.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.QueryRule", len)?;
        if let Some(v) = self.meta.as_ref() {
            struct_ser.serialize_field("meta", v)?;
        }
        if let Some(v) = self.query.as_ref() {
            struct_ser.serialize_field("query", v)?;
        }
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if !self.subject_var.is_empty() {
            struct_ser.serialize_field("subjectVar", &self.subject_var)?;
        }
        if !self.pin_var.is_empty() {
            struct_ser.serialize_field("pinVar", &self.pin_var)?;
        }
        if !self.message.is_empty() {
            struct_ser.serialize_field("message", &self.message)?;
        }
        if !self.param_symbol.is_empty() {
            struct_ser.serialize_field("paramSymbol", &self.param_symbol)?;
        }
        if !self.context_vars.is_empty() {
            struct_ser.serialize_field("contextVars", &self.context_vars)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for QueryRule {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "meta",
            "query",
            "kind",
            "subject_var",
            "subjectVar",
            "pin_var",
            "pinVar",
            "message",
            "param_symbol",
            "paramSymbol",
            "context_vars",
            "contextVars",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Meta,
            Query,
            Kind,
            SubjectVar,
            PinVar,
            Message,
            ParamSymbol,
            ContextVars,
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
                            "meta" => Ok(GeneratedField::Meta),
                            "query" => Ok(GeneratedField::Query),
                            "kind" => Ok(GeneratedField::Kind),
                            "subjectVar" | "subject_var" => Ok(GeneratedField::SubjectVar),
                            "pinVar" | "pin_var" => Ok(GeneratedField::PinVar),
                            "message" => Ok(GeneratedField::Message),
                            "paramSymbol" | "param_symbol" => Ok(GeneratedField::ParamSymbol),
                            "contextVars" | "context_vars" => Ok(GeneratedField::ContextVars),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = QueryRule;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.QueryRule")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<QueryRule, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut meta__ = None;
                let mut query__ = None;
                let mut kind__ = None;
                let mut subject_var__ = None;
                let mut pin_var__ = None;
                let mut message__ = None;
                let mut param_symbol__ = None;
                let mut context_vars__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Meta => {
                            if meta__.is_some() {
                                return Err(serde::de::Error::duplicate_field("meta"));
                            }
                            meta__ = map_.next_value()?;
                        }
                        GeneratedField::Query => {
                            if query__.is_some() {
                                return Err(serde::de::Error::duplicate_field("query"));
                            }
                            query__ = map_.next_value()?;
                        }
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::SubjectVar => {
                            if subject_var__.is_some() {
                                return Err(serde::de::Error::duplicate_field("subjectVar"));
                            }
                            subject_var__ = Some(map_.next_value()?);
                        }
                        GeneratedField::PinVar => {
                            if pin_var__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pinVar"));
                            }
                            pin_var__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Message => {
                            if message__.is_some() {
                                return Err(serde::de::Error::duplicate_field("message"));
                            }
                            message__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ParamSymbol => {
                            if param_symbol__.is_some() {
                                return Err(serde::de::Error::duplicate_field("paramSymbol"));
                            }
                            param_symbol__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ContextVars => {
                            if context_vars__.is_some() {
                                return Err(serde::de::Error::duplicate_field("contextVars"));
                            }
                            context_vars__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(QueryRule {
                    meta: meta__,
                    query: query__,
                    kind: kind__.unwrap_or_default(),
                    subject_var: subject_var__.unwrap_or_default(),
                    pin_var: pin_var__.unwrap_or_default(),
                    message: message__.unwrap_or_default(),
                    param_symbol: param_symbol__.unwrap_or_default(),
                    context_vars: context_vars__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.QueryRule", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ResultsMeta {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.schema.is_empty() {
            len += 1;
        }
        if !self.producer.is_empty() {
            len += 1;
        }
        if !self.producer_version.is_empty() {
            len += 1;
        }
        if !self.created_at.is_empty() {
            len += 1;
        }
        if self.coverage_axis {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ResultsMeta", len)?;
        if !self.schema.is_empty() {
            struct_ser.serialize_field("schema", &self.schema)?;
        }
        if !self.producer.is_empty() {
            struct_ser.serialize_field("producer", &self.producer)?;
        }
        if !self.producer_version.is_empty() {
            struct_ser.serialize_field("producerVersion", &self.producer_version)?;
        }
        if !self.created_at.is_empty() {
            struct_ser.serialize_field("createdAt", &self.created_at)?;
        }
        if self.coverage_axis {
            struct_ser.serialize_field("coverageAxis", &self.coverage_axis)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ResultsMeta {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "schema",
            "producer",
            "producer_version",
            "producerVersion",
            "created_at",
            "createdAt",
            "coverage_axis",
            "coverageAxis",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Schema,
            Producer,
            ProducerVersion,
            CreatedAt,
            CoverageAxis,
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
                            "schema" => Ok(GeneratedField::Schema),
                            "producer" => Ok(GeneratedField::Producer),
                            "producerVersion" | "producer_version" => Ok(GeneratedField::ProducerVersion),
                            "createdAt" | "created_at" => Ok(GeneratedField::CreatedAt),
                            "coverageAxis" | "coverage_axis" => Ok(GeneratedField::CoverageAxis),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ResultsMeta;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ResultsMeta")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ResultsMeta, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut schema__ = None;
                let mut producer__ = None;
                let mut producer_version__ = None;
                let mut created_at__ = None;
                let mut coverage_axis__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Schema => {
                            if schema__.is_some() {
                                return Err(serde::de::Error::duplicate_field("schema"));
                            }
                            schema__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Producer => {
                            if producer__.is_some() {
                                return Err(serde::de::Error::duplicate_field("producer"));
                            }
                            producer__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ProducerVersion => {
                            if producer_version__.is_some() {
                                return Err(serde::de::Error::duplicate_field("producerVersion"));
                            }
                            producer_version__ = Some(map_.next_value()?);
                        }
                        GeneratedField::CreatedAt => {
                            if created_at__.is_some() {
                                return Err(serde::de::Error::duplicate_field("createdAt"));
                            }
                            created_at__ = Some(map_.next_value()?);
                        }
                        GeneratedField::CoverageAxis => {
                            if coverage_axis__.is_some() {
                                return Err(serde::de::Error::duplicate_field("coverageAxis"));
                            }
                            coverage_axis__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ResultsMeta {
                    schema: schema__.unwrap_or_default(),
                    producer: producer__.unwrap_or_default(),
                    producer_version: producer_version__.unwrap_or_default(),
                    created_at: created_at__.unwrap_or_default(),
                    coverage_axis: coverage_axis__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ResultsMeta", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ReviewArea {
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
        if !self.items.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ReviewArea", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.items.is_empty() {
            struct_ser.serialize_field("items", &self.items)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ReviewArea {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "items",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Items,
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
                            "items" => Ok(GeneratedField::Items),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ReviewArea;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ReviewArea")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ReviewArea, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut items__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Items => {
                            if items__.is_some() {
                                return Err(serde::de::Error::duplicate_field("items"));
                            }
                            items__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ReviewArea {
                    name: name__.unwrap_or_default(),
                    items: items__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ReviewArea", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ReviewItem {
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
        if !self.outcome.is_empty() {
            len += 1;
        }
        if !self.note.is_empty() {
            len += 1;
        }
        if !self.findings.is_empty() {
            len += 1;
        }
        if !self.unmet.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ReviewItem", len)?;
        if !self.id.is_empty() {
            struct_ser.serialize_field("id", &self.id)?;
        }
        if !self.title.is_empty() {
            struct_ser.serialize_field("title", &self.title)?;
        }
        if !self.outcome.is_empty() {
            struct_ser.serialize_field("outcome", &self.outcome)?;
        }
        if !self.note.is_empty() {
            struct_ser.serialize_field("note", &self.note)?;
        }
        if !self.findings.is_empty() {
            struct_ser.serialize_field("findings", &self.findings)?;
        }
        if !self.unmet.is_empty() {
            struct_ser.serialize_field("unmet", &self.unmet)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ReviewItem {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "id",
            "title",
            "outcome",
            "note",
            "findings",
            "unmet",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Id,
            Title,
            Outcome,
            Note,
            Findings,
            Unmet,
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
                            "outcome" => Ok(GeneratedField::Outcome),
                            "note" => Ok(GeneratedField::Note),
                            "findings" => Ok(GeneratedField::Findings),
                            "unmet" => Ok(GeneratedField::Unmet),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ReviewItem;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ReviewItem")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ReviewItem, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut id__ = None;
                let mut title__ = None;
                let mut outcome__ = None;
                let mut note__ = None;
                let mut findings__ = None;
                let mut unmet__ = None;
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
                        GeneratedField::Outcome => {
                            if outcome__.is_some() {
                                return Err(serde::de::Error::duplicate_field("outcome"));
                            }
                            outcome__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Note => {
                            if note__.is_some() {
                                return Err(serde::de::Error::duplicate_field("note"));
                            }
                            note__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Findings => {
                            if findings__.is_some() {
                                return Err(serde::de::Error::duplicate_field("findings"));
                            }
                            findings__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Unmet => {
                            if unmet__.is_some() {
                                return Err(serde::de::Error::duplicate_field("unmet"));
                            }
                            unmet__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ReviewItem {
                    id: id__.unwrap_or_default(),
                    title: title__.unwrap_or_default(),
                    outcome: outcome__.unwrap_or_default(),
                    note: note__.unwrap_or_default(),
                    findings: findings__.unwrap_or_default(),
                    unmet: unmet__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ReviewItem", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ReviewManifest {
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
        if !self.areas.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.ReviewManifest", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.areas.is_empty() {
            struct_ser.serialize_field("areas", &self.areas)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ReviewManifest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "areas",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Areas,
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
                            "areas" => Ok(GeneratedField::Areas),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ReviewManifest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.ReviewManifest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ReviewManifest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut areas__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Areas => {
                            if areas__.is_some() {
                                return Err(serde::de::Error::duplicate_field("areas"));
                            }
                            areas__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ReviewManifest {
                    name: name__.unwrap_or_default(),
                    areas: areas__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.ReviewManifest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for RuleDeck {
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
        if !self.source.is_empty() {
            len += 1;
        }
        if !self.rules.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.RuleDeck", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.source.is_empty() {
            struct_ser.serialize_field("source", &self.source)?;
        }
        if !self.rules.is_empty() {
            struct_ser.serialize_field("rules", &self.rules)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for RuleDeck {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "source",
            "rules",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Source,
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
                            "source" => Ok(GeneratedField::Source),
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
            type Value = RuleDeck;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.RuleDeck")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<RuleDeck, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut source__ = None;
                let mut rules__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Source => {
                            if source__.is_some() {
                                return Err(serde::de::Error::duplicate_field("source"));
                            }
                            source__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Rules => {
                            if rules__.is_some() {
                                return Err(serde::de::Error::duplicate_field("rules"));
                            }
                            rules__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(RuleDeck {
                    name: name__.unwrap_or_default(),
                    source: source__.unwrap_or_default(),
                    rules: rules__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.RuleDeck", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for RuleDef {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.body.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.RuleDef", len)?;
        if let Some(v) = self.body.as_ref() {
            match v {
                rule_def::Body::Spec(v) => {
                    struct_ser.serialize_field("spec", v)?;
                }
                rule_def::Body::Query(v) => {
                    struct_ser.serialize_field("query", v)?;
                }
                rule_def::Body::Profile(v) => {
                    struct_ser.serialize_field("profile", v)?;
                }
            }
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for RuleDef {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "spec",
            "query",
            "profile",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Spec,
            Query,
            Profile,
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
                            "spec" => Ok(GeneratedField::Spec),
                            "query" => Ok(GeneratedField::Query),
                            "profile" => Ok(GeneratedField::Profile),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = RuleDef;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.RuleDef")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<RuleDef, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut body__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Spec => {
                            if body__.is_some() {
                                return Err(serde::de::Error::duplicate_field("spec"));
                            }
                            body__ = map_.next_value::<::std::option::Option<_>>()?.map(rule_def::Body::Spec)
;
                        }
                        GeneratedField::Query => {
                            if body__.is_some() {
                                return Err(serde::de::Error::duplicate_field("query"));
                            }
                            body__ = map_.next_value::<::std::option::Option<_>>()?.map(rule_def::Body::Query)
;
                        }
                        GeneratedField::Profile => {
                            if body__.is_some() {
                                return Err(serde::de::Error::duplicate_field("profile"));
                            }
                            body__ = map_.next_value::<::std::option::Option<_>>()?.map(rule_def::Body::Profile)
;
                        }
                    }
                }
                Ok(RuleDef {
                    body: body__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.RuleDef", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for RuleMeta {
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
        if !self.summary.is_empty() {
            len += 1;
        }
        if !self.impact.is_empty() {
            len += 1;
        }
        if !self.detail.is_empty() {
            len += 1;
        }
        if !self.tags.is_empty() {
            len += 1;
        }
        if !self.optional_reads.is_empty() {
            len += 1;
        }
        if !self.requires_capability.is_empty() {
            len += 1;
        }
        if !self.remedy.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.RuleMeta", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.severity.is_empty() {
            struct_ser.serialize_field("severity", &self.severity)?;
        }
        if !self.summary.is_empty() {
            struct_ser.serialize_field("summary", &self.summary)?;
        }
        if !self.impact.is_empty() {
            struct_ser.serialize_field("impact", &self.impact)?;
        }
        if !self.detail.is_empty() {
            struct_ser.serialize_field("detail", &self.detail)?;
        }
        if !self.tags.is_empty() {
            struct_ser.serialize_field("tags", &self.tags)?;
        }
        if !self.optional_reads.is_empty() {
            struct_ser.serialize_field("optionalReads", &self.optional_reads)?;
        }
        if !self.requires_capability.is_empty() {
            struct_ser.serialize_field("requiresCapability", &self.requires_capability)?;
        }
        if !self.remedy.is_empty() {
            struct_ser.serialize_field("remedy", &self.remedy)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for RuleMeta {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "severity",
            "summary",
            "impact",
            "detail",
            "tags",
            "optional_reads",
            "optionalReads",
            "requires_capability",
            "requiresCapability",
            "remedy",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Severity,
            Summary,
            Impact,
            Detail,
            Tags,
            OptionalReads,
            RequiresCapability,
            Remedy,
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
                            "summary" => Ok(GeneratedField::Summary),
                            "impact" => Ok(GeneratedField::Impact),
                            "detail" => Ok(GeneratedField::Detail),
                            "tags" => Ok(GeneratedField::Tags),
                            "optionalReads" | "optional_reads" => Ok(GeneratedField::OptionalReads),
                            "requiresCapability" | "requires_capability" => Ok(GeneratedField::RequiresCapability),
                            "remedy" => Ok(GeneratedField::Remedy),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = RuleMeta;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.RuleMeta")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<RuleMeta, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut severity__ = None;
                let mut summary__ = None;
                let mut impact__ = None;
                let mut detail__ = None;
                let mut tags__ = None;
                let mut optional_reads__ = None;
                let mut requires_capability__ = None;
                let mut remedy__ = None;
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
                        GeneratedField::Summary => {
                            if summary__.is_some() {
                                return Err(serde::de::Error::duplicate_field("summary"));
                            }
                            summary__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Impact => {
                            if impact__.is_some() {
                                return Err(serde::de::Error::duplicate_field("impact"));
                            }
                            impact__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Detail => {
                            if detail__.is_some() {
                                return Err(serde::de::Error::duplicate_field("detail"));
                            }
                            detail__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Tags => {
                            if tags__.is_some() {
                                return Err(serde::de::Error::duplicate_field("tags"));
                            }
                            tags__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                        GeneratedField::OptionalReads => {
                            if optional_reads__.is_some() {
                                return Err(serde::de::Error::duplicate_field("optionalReads"));
                            }
                            optional_reads__ = Some(map_.next_value()?);
                        }
                        GeneratedField::RequiresCapability => {
                            if requires_capability__.is_some() {
                                return Err(serde::de::Error::duplicate_field("requiresCapability"));
                            }
                            requires_capability__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Remedy => {
                            if remedy__.is_some() {
                                return Err(serde::de::Error::duplicate_field("remedy"));
                            }
                            remedy__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(RuleMeta {
                    name: name__.unwrap_or_default(),
                    severity: severity__.unwrap_or_default(),
                    summary: summary__.unwrap_or_default(),
                    impact: impact__.unwrap_or_default(),
                    detail: detail__.unwrap_or_default(),
                    tags: tags__.unwrap_or_default(),
                    optional_reads: optional_reads__.unwrap_or_default(),
                    requires_capability: requires_capability__.unwrap_or_default(),
                    remedy: remedy__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.RuleMeta", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for RuleRecord {
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
        if !self.summary.is_empty() {
            len += 1;
        }
        if !self.tags.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.RuleRecord", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.severity.is_empty() {
            struct_ser.serialize_field("severity", &self.severity)?;
        }
        if !self.summary.is_empty() {
            struct_ser.serialize_field("summary", &self.summary)?;
        }
        if !self.tags.is_empty() {
            struct_ser.serialize_field("tags", &self.tags)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for RuleRecord {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "severity",
            "summary",
            "tags",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
            Severity,
            Summary,
            Tags,
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
                            "summary" => Ok(GeneratedField::Summary),
                            "tags" => Ok(GeneratedField::Tags),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = RuleRecord;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.RuleRecord")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<RuleRecord, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut severity__ = None;
                let mut summary__ = None;
                let mut tags__ = None;
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
                        GeneratedField::Summary => {
                            if summary__.is_some() {
                                return Err(serde::de::Error::duplicate_field("summary"));
                            }
                            summary__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Tags => {
                            if tags__.is_some() {
                                return Err(serde::de::Error::duplicate_field("tags"));
                            }
                            tags__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                    }
                }
                Ok(RuleRecord {
                    name: name__.unwrap_or_default(),
                    severity: severity__.unwrap_or_default(),
                    summary: summary__.unwrap_or_default(),
                    tags: tags__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.RuleRecord", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for RunConfig {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.params {
            len += 1;
        }
        if self.profiles {
            len += 1;
        }
        if self.intent {
            len += 1;
        }
        if !self.conventions.is_empty() {
            len += 1;
        }
        if self.ratified_floor != 0. {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.RunConfig", len)?;
        if self.params {
            struct_ser.serialize_field("params", &self.params)?;
        }
        if self.profiles {
            struct_ser.serialize_field("profiles", &self.profiles)?;
        }
        if self.intent {
            struct_ser.serialize_field("intent", &self.intent)?;
        }
        if !self.conventions.is_empty() {
            struct_ser.serialize_field("conventions", &self.conventions)?;
        }
        if self.ratified_floor != 0. {
            struct_ser.serialize_field("ratifiedFloor", &self.ratified_floor)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for RunConfig {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "params",
            "profiles",
            "intent",
            "conventions",
            "ratified_floor",
            "ratifiedFloor",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Params,
            Profiles,
            Intent,
            Conventions,
            RatifiedFloor,
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
                            "params" => Ok(GeneratedField::Params),
                            "profiles" => Ok(GeneratedField::Profiles),
                            "intent" => Ok(GeneratedField::Intent),
                            "conventions" => Ok(GeneratedField::Conventions),
                            "ratifiedFloor" | "ratified_floor" => Ok(GeneratedField::RatifiedFloor),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = RunConfig;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.RunConfig")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<RunConfig, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut params__ = None;
                let mut profiles__ = None;
                let mut intent__ = None;
                let mut conventions__ = None;
                let mut ratified_floor__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Params => {
                            if params__.is_some() {
                                return Err(serde::de::Error::duplicate_field("params"));
                            }
                            params__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Profiles => {
                            if profiles__.is_some() {
                                return Err(serde::de::Error::duplicate_field("profiles"));
                            }
                            profiles__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Intent => {
                            if intent__.is_some() {
                                return Err(serde::de::Error::duplicate_field("intent"));
                            }
                            intent__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Conventions => {
                            if conventions__.is_some() {
                                return Err(serde::de::Error::duplicate_field("conventions"));
                            }
                            conventions__ = Some(map_.next_value()?);
                        }
                        GeneratedField::RatifiedFloor => {
                            if ratified_floor__.is_some() {
                                return Err(serde::de::Error::duplicate_field("ratifiedFloor"));
                            }
                            ratified_floor__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(RunConfig {
                    params: params__.unwrap_or_default(),
                    profiles: profiles__.unwrap_or_default(),
                    intent: intent__.unwrap_or_default(),
                    conventions: conventions__.unwrap_or_default(),
                    ratified_floor: ratified_floor__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.RunConfig", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SkippedRule {
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
        if !self.reason.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.SkippedRule", len)?;
        if !self.name.is_empty() {
            struct_ser.serialize_field("name", &self.name)?;
        }
        if !self.reason.is_empty() {
            struct_ser.serialize_field("reason", &self.reason)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SkippedRule {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "name",
            "reason",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Name,
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
            type Value = SkippedRule;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.SkippedRule")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SkippedRule, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut name__ = None;
                let mut reason__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Name => {
                            if name__.is_some() {
                                return Err(serde::de::Error::duplicate_field("name"));
                            }
                            name__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Reason => {
                            if reason__.is_some() {
                                return Err(serde::de::Error::duplicate_field("reason"));
                            }
                            reason__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(SkippedRule {
                    name: name__.unwrap_or_default(),
                    reason: reason__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.SkippedRule", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SpecBody {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.over.is_empty() {
            len += 1;
        }
        if !self.r#let.is_empty() {
            len += 1;
        }
        if self.r#where.is_some() {
            len += 1;
        }
        if !self.message.is_empty() {
            len += 1;
        }
        if self.scope.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.SpecBody", len)?;
        if !self.over.is_empty() {
            struct_ser.serialize_field("over", &self.over)?;
        }
        if !self.r#let.is_empty() {
            struct_ser.serialize_field("let", &self.r#let)?;
        }
        if let Some(v) = self.r#where.as_ref() {
            struct_ser.serialize_field("where", v)?;
        }
        if !self.message.is_empty() {
            struct_ser.serialize_field("message", &self.message)?;
        }
        if let Some(v) = self.scope.as_ref() {
            struct_ser.serialize_field("scope", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SpecBody {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "over",
            "let",
            "where",
            "message",
            "scope",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Over,
            Let,
            Where,
            Message,
            Scope,
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
                            "over" => Ok(GeneratedField::Over),
                            "let" => Ok(GeneratedField::Let),
                            "where" => Ok(GeneratedField::Where),
                            "message" => Ok(GeneratedField::Message),
                            "scope" => Ok(GeneratedField::Scope),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SpecBody;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.SpecBody")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SpecBody, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut over__ = None;
                let mut r#let__ = None;
                let mut r#where__ = None;
                let mut message__ = None;
                let mut scope__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Over => {
                            if over__.is_some() {
                                return Err(serde::de::Error::duplicate_field("over"));
                            }
                            over__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Let => {
                            if r#let__.is_some() {
                                return Err(serde::de::Error::duplicate_field("let"));
                            }
                            r#let__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                        GeneratedField::Where => {
                            if r#where__.is_some() {
                                return Err(serde::de::Error::duplicate_field("where"));
                            }
                            r#where__ = map_.next_value()?;
                        }
                        GeneratedField::Message => {
                            if message__.is_some() {
                                return Err(serde::de::Error::duplicate_field("message"));
                            }
                            message__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Scope => {
                            if scope__.is_some() {
                                return Err(serde::de::Error::duplicate_field("scope"));
                            }
                            scope__ = map_.next_value()?;
                        }
                    }
                }
                Ok(SpecBody {
                    over: over__.unwrap_or_default(),
                    r#let: r#let__.unwrap_or_default(),
                    r#where: r#where__,
                    message: message__.unwrap_or_default(),
                    scope: scope__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.SpecBody", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SpecCall {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.r#fn.is_empty() {
            len += 1;
        }
        if !self.args.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.SpecCall", len)?;
        if !self.r#fn.is_empty() {
            struct_ser.serialize_field("fn", &self.r#fn)?;
        }
        if !self.args.is_empty() {
            struct_ser.serialize_field("args", &self.args)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SpecCall {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "fn",
            "args",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Fn,
            Args,
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
                            "fn" => Ok(GeneratedField::Fn),
                            "args" => Ok(GeneratedField::Args),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SpecCall;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.SpecCall")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SpecCall, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut r#fn__ = None;
                let mut args__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Fn => {
                            if r#fn__.is_some() {
                                return Err(serde::de::Error::duplicate_field("fn"));
                            }
                            r#fn__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Args => {
                            if args__.is_some() {
                                return Err(serde::de::Error::duplicate_field("args"));
                            }
                            args__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(SpecCall {
                    r#fn: r#fn__.unwrap_or_default(),
                    args: args__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.SpecCall", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SpecCmp {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.l.is_some() {
            len += 1;
        }
        if !self.op.is_empty() {
            len += 1;
        }
        if self.r.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.SpecCmp", len)?;
        if let Some(v) = self.l.as_ref() {
            struct_ser.serialize_field("l", v)?;
        }
        if !self.op.is_empty() {
            struct_ser.serialize_field("op", &self.op)?;
        }
        if let Some(v) = self.r.as_ref() {
            struct_ser.serialize_field("r", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SpecCmp {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "l",
            "op",
            "r",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            L,
            Op,
            R,
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
                            "l" => Ok(GeneratedField::L),
                            "op" => Ok(GeneratedField::Op),
                            "r" => Ok(GeneratedField::R),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SpecCmp;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.SpecCmp")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SpecCmp, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut l__ = None;
                let mut op__ = None;
                let mut r__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::L => {
                            if l__.is_some() {
                                return Err(serde::de::Error::duplicate_field("l"));
                            }
                            l__ = map_.next_value()?;
                        }
                        GeneratedField::Op => {
                            if op__.is_some() {
                                return Err(serde::de::Error::duplicate_field("op"));
                            }
                            op__ = Some(map_.next_value()?);
                        }
                        GeneratedField::R => {
                            if r__.is_some() {
                                return Err(serde::de::Error::duplicate_field("r"));
                            }
                            r__ = map_.next_value()?;
                        }
                    }
                }
                Ok(SpecCmp {
                    l: l__,
                    op: op__.unwrap_or_default(),
                    r: r__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.SpecCmp", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SpecCountOf {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.over.is_empty() {
            len += 1;
        }
        if self.r#where.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.SpecCountOf", len)?;
        if !self.over.is_empty() {
            struct_ser.serialize_field("over", &self.over)?;
        }
        if let Some(v) = self.r#where.as_ref() {
            struct_ser.serialize_field("where", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SpecCountOf {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "over",
            "where",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Over,
            Where,
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
                            "over" => Ok(GeneratedField::Over),
                            "where" => Ok(GeneratedField::Where),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SpecCountOf;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.SpecCountOf")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SpecCountOf, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut over__ = None;
                let mut r#where__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Over => {
                            if over__.is_some() {
                                return Err(serde::de::Error::duplicate_field("over"));
                            }
                            over__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Where => {
                            if r#where__.is_some() {
                                return Err(serde::de::Error::duplicate_field("where"));
                            }
                            r#where__ = map_.next_value()?;
                        }
                    }
                }
                Ok(SpecCountOf {
                    over: over__.unwrap_or_default(),
                    r#where: r#where__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.SpecCountOf", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SpecExistsIn {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.over.is_empty() {
            len += 1;
        }
        if self.r#where.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.SpecExistsIn", len)?;
        if !self.over.is_empty() {
            struct_ser.serialize_field("over", &self.over)?;
        }
        if let Some(v) = self.r#where.as_ref() {
            struct_ser.serialize_field("where", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SpecExistsIn {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "over",
            "where",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Over,
            Where,
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
                            "over" => Ok(GeneratedField::Over),
                            "where" => Ok(GeneratedField::Where),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SpecExistsIn;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.SpecExistsIn")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SpecExistsIn, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut over__ = None;
                let mut r#where__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Over => {
                            if over__.is_some() {
                                return Err(serde::de::Error::duplicate_field("over"));
                            }
                            over__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Where => {
                            if r#where__.is_some() {
                                return Err(serde::de::Error::duplicate_field("where"));
                            }
                            r#where__ = map_.next_value()?;
                        }
                    }
                }
                Ok(SpecExistsIn {
                    over: over__.unwrap_or_default(),
                    r#where: r#where__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.SpecExistsIn", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SpecExpr {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.expr.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.SpecExpr", len)?;
        if let Some(v) = self.expr.as_ref() {
            match v {
                spec_expr::Expr::And(v) => {
                    struct_ser.serialize_field("and", v)?;
                }
                spec_expr::Expr::Or(v) => {
                    struct_ser.serialize_field("or", v)?;
                }
                spec_expr::Expr::Not(v) => {
                    struct_ser.serialize_field("not", v)?;
                }
                spec_expr::Expr::Cmp(v) => {
                    struct_ser.serialize_field("cmp", v)?;
                }
                spec_expr::Expr::In(v) => {
                    struct_ser.serialize_field("in", v)?;
                }
                spec_expr::Expr::Match(v) => {
                    struct_ser.serialize_field("match", v)?;
                }
                spec_expr::Expr::ExistsIn(v) => {
                    struct_ser.serialize_field("existsIn", v)?;
                }
                spec_expr::Expr::IsTrue(v) => {
                    struct_ser.serialize_field("isTrue", v)?;
                }
            }
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SpecExpr {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "and",
            "or",
            "not",
            "cmp",
            "in",
            "match",
            "exists_in",
            "existsIn",
            "is_true",
            "isTrue",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            And,
            Or,
            Not,
            Cmp,
            In,
            Match,
            ExistsIn,
            IsTrue,
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
                            "and" => Ok(GeneratedField::And),
                            "or" => Ok(GeneratedField::Or),
                            "not" => Ok(GeneratedField::Not),
                            "cmp" => Ok(GeneratedField::Cmp),
                            "in" => Ok(GeneratedField::In),
                            "match" => Ok(GeneratedField::Match),
                            "existsIn" | "exists_in" => Ok(GeneratedField::ExistsIn),
                            "isTrue" | "is_true" => Ok(GeneratedField::IsTrue),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SpecExpr;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.SpecExpr")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SpecExpr, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut expr__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::And => {
                            if expr__.is_some() {
                                return Err(serde::de::Error::duplicate_field("and"));
                            }
                            expr__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_expr::Expr::And)
;
                        }
                        GeneratedField::Or => {
                            if expr__.is_some() {
                                return Err(serde::de::Error::duplicate_field("or"));
                            }
                            expr__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_expr::Expr::Or)
;
                        }
                        GeneratedField::Not => {
                            if expr__.is_some() {
                                return Err(serde::de::Error::duplicate_field("not"));
                            }
                            expr__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_expr::Expr::Not)
;
                        }
                        GeneratedField::Cmp => {
                            if expr__.is_some() {
                                return Err(serde::de::Error::duplicate_field("cmp"));
                            }
                            expr__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_expr::Expr::Cmp)
;
                        }
                        GeneratedField::In => {
                            if expr__.is_some() {
                                return Err(serde::de::Error::duplicate_field("in"));
                            }
                            expr__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_expr::Expr::In)
;
                        }
                        GeneratedField::Match => {
                            if expr__.is_some() {
                                return Err(serde::de::Error::duplicate_field("match"));
                            }
                            expr__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_expr::Expr::Match)
;
                        }
                        GeneratedField::ExistsIn => {
                            if expr__.is_some() {
                                return Err(serde::de::Error::duplicate_field("existsIn"));
                            }
                            expr__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_expr::Expr::ExistsIn)
;
                        }
                        GeneratedField::IsTrue => {
                            if expr__.is_some() {
                                return Err(serde::de::Error::duplicate_field("isTrue"));
                            }
                            expr__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_expr::Expr::IsTrue)
;
                        }
                    }
                }
                Ok(SpecExpr {
                    expr: expr__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.SpecExpr", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SpecExprList {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.xs.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.SpecExprList", len)?;
        if !self.xs.is_empty() {
            struct_ser.serialize_field("xs", &self.xs)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SpecExprList {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "xs",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Xs,
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
                            "xs" => Ok(GeneratedField::Xs),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SpecExprList;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.SpecExprList")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SpecExprList, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut xs__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Xs => {
                            if xs__.is_some() {
                                return Err(serde::de::Error::duplicate_field("xs"));
                            }
                            xs__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(SpecExprList {
                    xs: xs__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.SpecExprList", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SpecIn {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.t.is_some() {
            len += 1;
        }
        if !self.set.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.SpecIn", len)?;
        if let Some(v) = self.t.as_ref() {
            struct_ser.serialize_field("t", v)?;
        }
        if !self.set.is_empty() {
            struct_ser.serialize_field("set", &self.set)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SpecIn {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "t",
            "set",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            T,
            Set,
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
                            "t" => Ok(GeneratedField::T),
                            "set" => Ok(GeneratedField::Set),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SpecIn;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.SpecIn")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SpecIn, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut t__ = None;
                let mut set__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::T => {
                            if t__.is_some() {
                                return Err(serde::de::Error::duplicate_field("t"));
                            }
                            t__ = map_.next_value()?;
                        }
                        GeneratedField::Set => {
                            if set__.is_some() {
                                return Err(serde::de::Error::duplicate_field("set"));
                            }
                            set__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(SpecIn {
                    t: t__,
                    set: set__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.SpecIn", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SpecLit {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.value.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.SpecLit", len)?;
        if let Some(v) = self.value.as_ref() {
            match v {
                spec_lit::Value::S(v) => {
                    struct_ser.serialize_field("s", v)?;
                }
                spec_lit::Value::I(v) => {
                    #[allow(clippy::needless_borrow)]
                    #[allow(clippy::needless_borrows_for_generic_args)]
                    struct_ser.serialize_field("i", ToString::to_string(&v).as_str())?;
                }
                spec_lit::Value::B(v) => {
                    struct_ser.serialize_field("b", v)?;
                }
            }
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SpecLit {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "s",
            "i",
            "b",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            S,
            I,
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
                            "s" => Ok(GeneratedField::S),
                            "i" => Ok(GeneratedField::I),
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
            type Value = SpecLit;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.SpecLit")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SpecLit, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut value__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::S => {
                            if value__.is_some() {
                                return Err(serde::de::Error::duplicate_field("s"));
                            }
                            value__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_lit::Value::S);
                        }
                        GeneratedField::I => {
                            if value__.is_some() {
                                return Err(serde::de::Error::duplicate_field("i"));
                            }
                            value__ = map_.next_value::<::std::option::Option<::pbjson::private::NumberDeserialize<_>>>()?.map(|x| spec_lit::Value::I(x.0));
                        }
                        GeneratedField::B => {
                            if value__.is_some() {
                                return Err(serde::de::Error::duplicate_field("b"));
                            }
                            value__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_lit::Value::B);
                        }
                    }
                }
                Ok(SpecLit {
                    value: value__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.SpecLit", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SpecMatch {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.t.is_some() {
            len += 1;
        }
        if !self.pattern.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.SpecMatch", len)?;
        if let Some(v) = self.t.as_ref() {
            struct_ser.serialize_field("t", v)?;
        }
        if !self.pattern.is_empty() {
            struct_ser.serialize_field("pattern", &self.pattern)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SpecMatch {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "t",
            "pattern",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            T,
            Pattern,
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
                            "t" => Ok(GeneratedField::T),
                            "pattern" => Ok(GeneratedField::Pattern),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SpecMatch;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.SpecMatch")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SpecMatch, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut t__ = None;
                let mut pattern__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::T => {
                            if t__.is_some() {
                                return Err(serde::de::Error::duplicate_field("t"));
                            }
                            t__ = map_.next_value()?;
                        }
                        GeneratedField::Pattern => {
                            if pattern__.is_some() {
                                return Err(serde::de::Error::duplicate_field("pattern"));
                            }
                            pattern__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(SpecMatch {
                    t: t__,
                    pattern: pattern__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.SpecMatch", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SpecRule {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.meta.is_some() {
            len += 1;
        }
        if self.body.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.SpecRule", len)?;
        if let Some(v) = self.meta.as_ref() {
            struct_ser.serialize_field("meta", v)?;
        }
        if let Some(v) = self.body.as_ref() {
            struct_ser.serialize_field("body", v)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SpecRule {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "meta",
            "body",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Meta,
            Body,
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
                            "meta" => Ok(GeneratedField::Meta),
                            "body" => Ok(GeneratedField::Body),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SpecRule;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.SpecRule")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SpecRule, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut meta__ = None;
                let mut body__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Meta => {
                            if meta__.is_some() {
                                return Err(serde::de::Error::duplicate_field("meta"));
                            }
                            meta__ = map_.next_value()?;
                        }
                        GeneratedField::Body => {
                            if body__.is_some() {
                                return Err(serde::de::Error::duplicate_field("body"));
                            }
                            body__ = map_.next_value()?;
                        }
                    }
                }
                Ok(SpecRule {
                    meta: meta__,
                    body: body__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.SpecRule", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SpecTerm {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.term.is_some() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.SpecTerm", len)?;
        if let Some(v) = self.term.as_ref() {
            match v {
                spec_term::Term::Lit(v) => {
                    struct_ser.serialize_field("lit", v)?;
                }
                spec_term::Term::Fact(v) => {
                    struct_ser.serialize_field("fact", v)?;
                }
                spec_term::Term::Var(v) => {
                    struct_ser.serialize_field("var", v)?;
                }
                spec_term::Term::Call(v) => {
                    struct_ser.serialize_field("call", v)?;
                }
                spec_term::Term::CountOf(v) => {
                    struct_ser.serialize_field("countOf", v)?;
                }
            }
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SpecTerm {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "lit",
            "fact",
            "var",
            "call",
            "count_of",
            "countOf",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Lit,
            Fact,
            Var,
            Call,
            CountOf,
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
                            "lit" => Ok(GeneratedField::Lit),
                            "fact" => Ok(GeneratedField::Fact),
                            "var" => Ok(GeneratedField::Var),
                            "call" => Ok(GeneratedField::Call),
                            "countOf" | "count_of" => Ok(GeneratedField::CountOf),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SpecTerm;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.SpecTerm")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<SpecTerm, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut term__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Lit => {
                            if term__.is_some() {
                                return Err(serde::de::Error::duplicate_field("lit"));
                            }
                            term__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_term::Term::Lit)
;
                        }
                        GeneratedField::Fact => {
                            if term__.is_some() {
                                return Err(serde::de::Error::duplicate_field("fact"));
                            }
                            term__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_term::Term::Fact);
                        }
                        GeneratedField::Var => {
                            if term__.is_some() {
                                return Err(serde::de::Error::duplicate_field("var"));
                            }
                            term__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_term::Term::Var);
                        }
                        GeneratedField::Call => {
                            if term__.is_some() {
                                return Err(serde::de::Error::duplicate_field("call"));
                            }
                            term__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_term::Term::Call)
;
                        }
                        GeneratedField::CountOf => {
                            if term__.is_some() {
                                return Err(serde::de::Error::duplicate_field("countOf"));
                            }
                            term__ = map_.next_value::<::std::option::Option<_>>()?.map(spec_term::Term::CountOf)
;
                        }
                    }
                }
                Ok(SpecTerm {
                    term: term__,
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.SpecTerm", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Subject {
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
        if !self.r#ref.is_empty() {
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
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.Subject", len)?;
        if !self.kind.is_empty() {
            struct_ser.serialize_field("kind", &self.kind)?;
        }
        if !self.r#ref.is_empty() {
            struct_ser.serialize_field("ref", &self.r#ref)?;
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
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Subject {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "kind",
            "ref",
            "pin",
            "net_id",
            "netId",
            "bus_id",
            "busId",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Kind,
            Ref,
            Pin,
            NetId,
            BusId,
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
                            "ref" => Ok(GeneratedField::Ref),
                            "pin" => Ok(GeneratedField::Pin),
                            "netId" | "net_id" => Ok(GeneratedField::NetId),
                            "busId" | "bus_id" => Ok(GeneratedField::BusId),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Subject;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.Subject")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Subject, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut kind__ = None;
                let mut r#ref__ = None;
                let mut pin__ = None;
                let mut net_id__ = None;
                let mut bus_id__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Kind => {
                            if kind__.is_some() {
                                return Err(serde::de::Error::duplicate_field("kind"));
                            }
                            kind__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Ref => {
                            if r#ref__.is_some() {
                                return Err(serde::de::Error::duplicate_field("ref"));
                            }
                            r#ref__ = Some(map_.next_value()?);
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
                    }
                }
                Ok(Subject {
                    kind: kind__.unwrap_or_default(),
                    r#ref: r#ref__.unwrap_or_default(),
                    pin: pin__.unwrap_or_default(),
                    net_id: net_id__.unwrap_or_default(),
                    bus_id: bus_id__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.Subject", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for UnjoinedReason {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.reason.is_empty() {
            len += 1;
        }
        if self.count != 0 {
            len += 1;
        }
        if !self.examples.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.UnjoinedReason", len)?;
        if !self.reason.is_empty() {
            struct_ser.serialize_field("reason", &self.reason)?;
        }
        if self.count != 0 {
            struct_ser.serialize_field("count", &self.count)?;
        }
        if !self.examples.is_empty() {
            struct_ser.serialize_field("examples", &self.examples)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for UnjoinedReason {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "reason",
            "count",
            "examples",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Reason,
            Count,
            Examples,
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
                            "reason" => Ok(GeneratedField::Reason),
                            "count" => Ok(GeneratedField::Count),
                            "examples" => Ok(GeneratedField::Examples),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = UnjoinedReason;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.UnjoinedReason")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<UnjoinedReason, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut reason__ = None;
                let mut count__ = None;
                let mut examples__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Reason => {
                            if reason__.is_some() {
                                return Err(serde::de::Error::duplicate_field("reason"));
                            }
                            reason__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Count => {
                            if count__.is_some() {
                                return Err(serde::de::Error::duplicate_field("count"));
                            }
                            count__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Examples => {
                            if examples__.is_some() {
                                return Err(serde::de::Error::duplicate_field("examples"));
                            }
                            examples__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(UnjoinedReason {
                    reason: reason__.unwrap_or_default(),
                    count: count__.unwrap_or_default(),
                    examples: examples__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.UnjoinedReason", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for UnmetDependency {
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
        if !self.symbol.is_empty() {
            len += 1;
        }
        if self.spec_absent {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.UnmetDependency", len)?;
        if !self.mpn.is_empty() {
            struct_ser.serialize_field("mpn", &self.mpn)?;
        }
        if !self.manufacturer.is_empty() {
            struct_ser.serialize_field("manufacturer", &self.manufacturer)?;
        }
        if !self.symbol.is_empty() {
            struct_ser.serialize_field("symbol", &self.symbol)?;
        }
        if self.spec_absent {
            struct_ser.serialize_field("specAbsent", &self.spec_absent)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for UnmetDependency {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "mpn",
            "manufacturer",
            "symbol",
            "spec_absent",
            "specAbsent",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Mpn,
            Manufacturer,
            Symbol,
            SpecAbsent,
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
                            "symbol" => Ok(GeneratedField::Symbol),
                            "specAbsent" | "spec_absent" => Ok(GeneratedField::SpecAbsent),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = UnmetDependency;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.UnmetDependency")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<UnmetDependency, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut mpn__ = None;
                let mut manufacturer__ = None;
                let mut symbol__ = None;
                let mut spec_absent__ = None;
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
                        GeneratedField::Symbol => {
                            if symbol__.is_some() {
                                return Err(serde::de::Error::duplicate_field("symbol"));
                            }
                            symbol__ = Some(map_.next_value()?);
                        }
                        GeneratedField::SpecAbsent => {
                            if spec_absent__.is_some() {
                                return Err(serde::de::Error::duplicate_field("specAbsent"));
                            }
                            spec_absent__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(UnmetDependency {
                    mpn: mpn__.unwrap_or_default(),
                    manufacturer: manufacturer__.unwrap_or_default(),
                    symbol: symbol__.unwrap_or_default(),
                    spec_absent: spec_absent__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.UnmetDependency", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Verdict {
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
        if self.outcome != 0 {
            len += 1;
        }
        if !self.subjects.is_empty() {
            len += 1;
        }
        if self.witness.is_some() {
            len += 1;
        }
        if !self.reason.is_empty() {
            len += 1;
        }
        if !self.context.is_empty() {
            len += 1;
        }
        if !self.sheets.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.Verdict", len)?;
        if !self.id.is_empty() {
            struct_ser.serialize_field("id", &self.id)?;
        }
        if !self.rule.is_empty() {
            struct_ser.serialize_field("rule", &self.rule)?;
        }
        if self.outcome != 0 {
            let v = Outcome::try_from(self.outcome)
                .map_err(|_| serde::ser::Error::custom(format!("Invalid variant {}", self.outcome)))?;
            struct_ser.serialize_field("outcome", &v)?;
        }
        if !self.subjects.is_empty() {
            struct_ser.serialize_field("subjects", &self.subjects)?;
        }
        if let Some(v) = self.witness.as_ref() {
            struct_ser.serialize_field("witness", v)?;
        }
        if !self.reason.is_empty() {
            struct_ser.serialize_field("reason", &self.reason)?;
        }
        if !self.context.is_empty() {
            struct_ser.serialize_field("context", &self.context)?;
        }
        if !self.sheets.is_empty() {
            struct_ser.serialize_field("sheets", &self.sheets)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Verdict {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "id",
            "rule",
            "outcome",
            "subjects",
            "witness",
            "reason",
            "context",
            "sheets",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Id,
            Rule,
            Outcome,
            Subjects,
            Witness,
            Reason,
            Context,
            Sheets,
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
                            "outcome" => Ok(GeneratedField::Outcome),
                            "subjects" => Ok(GeneratedField::Subjects),
                            "witness" => Ok(GeneratedField::Witness),
                            "reason" => Ok(GeneratedField::Reason),
                            "context" => Ok(GeneratedField::Context),
                            "sheets" => Ok(GeneratedField::Sheets),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Verdict;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.Verdict")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Verdict, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut id__ = None;
                let mut rule__ = None;
                let mut outcome__ = None;
                let mut subjects__ = None;
                let mut witness__ = None;
                let mut reason__ = None;
                let mut context__ = None;
                let mut sheets__ = None;
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
                        GeneratedField::Outcome => {
                            if outcome__.is_some() {
                                return Err(serde::de::Error::duplicate_field("outcome"));
                            }
                            outcome__ = Some(map_.next_value::<Outcome>()? as i32);
                        }
                        GeneratedField::Subjects => {
                            if subjects__.is_some() {
                                return Err(serde::de::Error::duplicate_field("subjects"));
                            }
                            subjects__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Witness => {
                            if witness__.is_some() {
                                return Err(serde::de::Error::duplicate_field("witness"));
                            }
                            witness__ = map_.next_value()?;
                        }
                        GeneratedField::Reason => {
                            if reason__.is_some() {
                                return Err(serde::de::Error::duplicate_field("reason"));
                            }
                            reason__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Context => {
                            if context__.is_some() {
                                return Err(serde::de::Error::duplicate_field("context"));
                            }
                            context__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Sheets => {
                            if sheets__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sheets"));
                            }
                            sheets__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Verdict {
                    id: id__.unwrap_or_default(),
                    rule: rule__.unwrap_or_default(),
                    outcome: outcome__.unwrap_or_default(),
                    subjects: subjects__.unwrap_or_default(),
                    witness: witness__,
                    reason: reason__.unwrap_or_default(),
                    context: context__.unwrap_or_default(),
                    sheets: sheets__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.Verdict", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for Witness {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.statement.is_empty() {
            len += 1;
        }
        if !self.terms.is_empty() {
            len += 1;
        }
        if !self.datasheet.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.Witness", len)?;
        if !self.statement.is_empty() {
            struct_ser.serialize_field("statement", &self.statement)?;
        }
        if !self.terms.is_empty() {
            struct_ser.serialize_field("terms", &self.terms)?;
        }
        if !self.datasheet.is_empty() {
            struct_ser.serialize_field("datasheet", &self.datasheet)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for Witness {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "statement",
            "terms",
            "datasheet",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Statement,
            Terms,
            Datasheet,
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
                            "statement" => Ok(GeneratedField::Statement),
                            "terms" => Ok(GeneratedField::Terms),
                            "datasheet" => Ok(GeneratedField::Datasheet),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = Witness;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.Witness")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<Witness, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut statement__ = None;
                let mut terms__ = None;
                let mut datasheet__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Statement => {
                            if statement__.is_some() {
                                return Err(serde::de::Error::duplicate_field("statement"));
                            }
                            statement__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Terms => {
                            if terms__.is_some() {
                                return Err(serde::de::Error::duplicate_field("terms"));
                            }
                            terms__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Datasheet => {
                            if datasheet__.is_some() {
                                return Err(serde::de::Error::duplicate_field("datasheet"));
                            }
                            datasheet__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(Witness {
                    statement: statement__.unwrap_or_default(),
                    terms: terms__.unwrap_or_default(),
                    datasheet: datasheet__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.Witness", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for WitnessTerm {
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
        if !self.value.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.WitnessTerm", len)?;
        if !self.label.is_empty() {
            struct_ser.serialize_field("label", &self.label)?;
        }
        if !self.value.is_empty() {
            struct_ser.serialize_field("value", &self.value)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for WitnessTerm {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "label",
            "value",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Label,
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
                            "label" => Ok(GeneratedField::Label),
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
            type Value = WitnessTerm;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.WitnessTerm")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<WitnessTerm, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut label__ = None;
                let mut value__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Label => {
                            if label__.is_some() {
                                return Err(serde::de::Error::duplicate_field("label"));
                            }
                            label__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Value => {
                            if value__.is_some() {
                                return Err(serde::de::Error::duplicate_field("value"));
                            }
                            value__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(WitnessTerm {
                    label: label__.unwrap_or_default(),
                    value: value__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.WitnessTerm", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for WorkItem {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.dependency.is_some() {
            len += 1;
        }
        if !self.blocked.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("agni.v1.checks.WorkItem", len)?;
        if let Some(v) = self.dependency.as_ref() {
            struct_ser.serialize_field("dependency", v)?;
        }
        if !self.blocked.is_empty() {
            struct_ser.serialize_field("blocked", &self.blocked)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for WorkItem {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "dependency",
            "blocked",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Dependency,
            Blocked,
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
                            "dependency" => Ok(GeneratedField::Dependency),
                            "blocked" => Ok(GeneratedField::Blocked),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = WorkItem;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct agni.v1.checks.WorkItem")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<WorkItem, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut dependency__ = None;
                let mut blocked__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Dependency => {
                            if dependency__.is_some() {
                                return Err(serde::de::Error::duplicate_field("dependency"));
                            }
                            dependency__ = map_.next_value()?;
                        }
                        GeneratedField::Blocked => {
                            if blocked__.is_some() {
                                return Err(serde::de::Error::duplicate_field("blocked"));
                            }
                            blocked__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(WorkItem {
                    dependency: dependency__,
                    blocked: blocked__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("agni.v1.checks.WorkItem", FIELDS, GeneratedVisitor)
    }
}
