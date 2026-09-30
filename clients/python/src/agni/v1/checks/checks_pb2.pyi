from agni.v1.ir import ir_pb2 as _ir_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class LocateReason(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    LOCATE_REASON_UNSPECIFIED: _ClassVar[LocateReason]
    LOCATE_REASON_VIRTUAL_SYMBOL: _ClassVar[LocateReason]
    LOCATE_REASON_POWER_RAIL_NO_WIRE: _ClassVar[LocateReason]
    LOCATE_REASON_NOT_IN_DESIGN: _ClassVar[LocateReason]
    LOCATE_REASON_NO_GEOMETRY: _ClassVar[LocateReason]
    LOCATE_REASON_BUS_NOT_DRAWN: _ClassVar[LocateReason]

class Outcome(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    OUTCOME_UNSPECIFIED: _ClassVar[Outcome]
    OUTCOME_PASS: _ClassVar[Outcome]
    OUTCOME_FAIL: _ClassVar[Outcome]
    OUTCOME_NO_LIMIT: _ClassVar[Outcome]
    OUTCOME_NOT_CONSIDERED: _ClassVar[Outcome]
    OUTCOME_INCONCLUSIVE: _ClassVar[Outcome]
LOCATE_REASON_UNSPECIFIED: LocateReason
LOCATE_REASON_VIRTUAL_SYMBOL: LocateReason
LOCATE_REASON_POWER_RAIL_NO_WIRE: LocateReason
LOCATE_REASON_NOT_IN_DESIGN: LocateReason
LOCATE_REASON_NO_GEOMETRY: LocateReason
LOCATE_REASON_BUS_NOT_DRAWN: LocateReason
OUTCOME_UNSPECIFIED: Outcome
OUTCOME_PASS: Outcome
OUTCOME_FAIL: Outcome
OUTCOME_NO_LIMIT: Outcome
OUTCOME_NOT_CONSIDERED: Outcome
OUTCOME_INCONCLUSIVE: Outcome

class Subject(_message.Message):
    __slots__ = ("kind", "ref", "pin", "net_id", "bus_id")
    KIND_FIELD_NUMBER: _ClassVar[int]
    REF_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    NET_ID_FIELD_NUMBER: _ClassVar[int]
    BUS_ID_FIELD_NUMBER: _ClassVar[int]
    kind: str
    ref: str
    pin: str
    net_id: str
    bus_id: str
    def __init__(self, kind: _Optional[str] = ..., ref: _Optional[str] = ..., pin: _Optional[str] = ..., net_id: _Optional[str] = ..., bus_id: _Optional[str] = ...) -> None: ...

class ContextSubject(_message.Message):
    __slots__ = ("subject", "role")
    SUBJECT_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    subject: Subject
    role: str
    def __init__(self, subject: _Optional[_Union[Subject, _Mapping]] = ..., role: _Optional[str] = ...) -> None: ...

class DatasheetCitation(_message.Message):
    __slots__ = ("doc", "doc_ref", "page", "section", "method", "confidence", "verification", "verified_revision")
    DOC_FIELD_NUMBER: _ClassVar[int]
    DOC_REF_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    SECTION_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    VERIFICATION_FIELD_NUMBER: _ClassVar[int]
    VERIFIED_REVISION_FIELD_NUMBER: _ClassVar[int]
    doc: str
    doc_ref: str
    page: int
    section: str
    method: str
    confidence: float
    verification: str
    verified_revision: str
    def __init__(self, doc: _Optional[str] = ..., doc_ref: _Optional[str] = ..., page: _Optional[int] = ..., section: _Optional[str] = ..., method: _Optional[str] = ..., confidence: _Optional[float] = ..., verification: _Optional[str] = ..., verified_revision: _Optional[str] = ...) -> None: ...

class Finding(_message.Message):
    __slots__ = ("rule", "severity", "subject", "message", "inconclusive", "provenance", "sheets", "locate_reason", "datasheets", "context")
    RULE_FIELD_NUMBER: _ClassVar[int]
    SEVERITY_FIELD_NUMBER: _ClassVar[int]
    SUBJECT_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    INCONCLUSIVE_FIELD_NUMBER: _ClassVar[int]
    PROVENANCE_FIELD_NUMBER: _ClassVar[int]
    SHEETS_FIELD_NUMBER: _ClassVar[int]
    LOCATE_REASON_FIELD_NUMBER: _ClassVar[int]
    DATASHEETS_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    rule: str
    severity: str
    subject: Subject
    message: str
    inconclusive: bool
    provenance: _ir_pb2.Provenance
    sheets: _containers.RepeatedScalarFieldContainer[str]
    locate_reason: LocateReason
    datasheets: _containers.RepeatedCompositeFieldContainer[DatasheetCitation]
    context: _containers.RepeatedCompositeFieldContainer[ContextSubject]
    def __init__(self, rule: _Optional[str] = ..., severity: _Optional[str] = ..., subject: _Optional[_Union[Subject, _Mapping]] = ..., message: _Optional[str] = ..., inconclusive: _Optional[bool] = ..., provenance: _Optional[_Union[_ir_pb2.Provenance, _Mapping]] = ..., sheets: _Optional[_Iterable[str]] = ..., locate_reason: _Optional[_Union[LocateReason, str]] = ..., datasheets: _Optional[_Iterable[_Union[DatasheetCitation, _Mapping]]] = ..., context: _Optional[_Iterable[_Union[ContextSubject, _Mapping]]] = ...) -> None: ...

class WitnessTerm(_message.Message):
    __slots__ = ("label", "value")
    LABEL_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    label: str
    value: str
    def __init__(self, label: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...

class Witness(_message.Message):
    __slots__ = ("statement", "terms", "datasheet")
    STATEMENT_FIELD_NUMBER: _ClassVar[int]
    TERMS_FIELD_NUMBER: _ClassVar[int]
    DATASHEET_FIELD_NUMBER: _ClassVar[int]
    statement: str
    terms: _containers.RepeatedCompositeFieldContainer[WitnessTerm]
    datasheet: _containers.RepeatedCompositeFieldContainer[DatasheetCitation]
    def __init__(self, statement: _Optional[str] = ..., terms: _Optional[_Iterable[_Union[WitnessTerm, _Mapping]]] = ..., datasheet: _Optional[_Iterable[_Union[DatasheetCitation, _Mapping]]] = ...) -> None: ...

class Verdict(_message.Message):
    __slots__ = ("id", "rule", "outcome", "subjects", "witness", "reason", "context")
    ID_FIELD_NUMBER: _ClassVar[int]
    RULE_FIELD_NUMBER: _ClassVar[int]
    OUTCOME_FIELD_NUMBER: _ClassVar[int]
    SUBJECTS_FIELD_NUMBER: _ClassVar[int]
    WITNESS_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    id: str
    rule: str
    outcome: Outcome
    subjects: _containers.RepeatedCompositeFieldContainer[Subject]
    witness: Witness
    reason: str
    context: _containers.RepeatedCompositeFieldContainer[ContextSubject]
    def __init__(self, id: _Optional[str] = ..., rule: _Optional[str] = ..., outcome: _Optional[_Union[Outcome, str]] = ..., subjects: _Optional[_Iterable[_Union[Subject, _Mapping]]] = ..., witness: _Optional[_Union[Witness, _Mapping]] = ..., reason: _Optional[str] = ..., context: _Optional[_Iterable[_Union[ContextSubject, _Mapping]]] = ...) -> None: ...

class CheckReport(_message.Message):
    __slots__ = ("source", "rules_run", "sections")
    class SeveritySection(_message.Message):
        __slots__ = ("severity", "count", "rules")
        SEVERITY_FIELD_NUMBER: _ClassVar[int]
        COUNT_FIELD_NUMBER: _ClassVar[int]
        RULES_FIELD_NUMBER: _ClassVar[int]
        severity: str
        count: int
        rules: _containers.RepeatedCompositeFieldContainer[CheckReport.RuleGroup]
        def __init__(self, severity: _Optional[str] = ..., count: _Optional[int] = ..., rules: _Optional[_Iterable[_Union[CheckReport.RuleGroup, _Mapping]]] = ...) -> None: ...
    class RuleGroup(_message.Message):
        __slots__ = ("rule", "summary", "findings")
        RULE_FIELD_NUMBER: _ClassVar[int]
        SUMMARY_FIELD_NUMBER: _ClassVar[int]
        FINDINGS_FIELD_NUMBER: _ClassVar[int]
        rule: str
        summary: str
        findings: _containers.RepeatedCompositeFieldContainer[Finding]
        def __init__(self, rule: _Optional[str] = ..., summary: _Optional[str] = ..., findings: _Optional[_Iterable[_Union[Finding, _Mapping]]] = ...) -> None: ...
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    RULES_RUN_FIELD_NUMBER: _ClassVar[int]
    SECTIONS_FIELD_NUMBER: _ClassVar[int]
    source: str
    rules_run: int
    sections: _containers.RepeatedCompositeFieldContainer[CheckReport.SeveritySection]
    def __init__(self, source: _Optional[str] = ..., rules_run: _Optional[int] = ..., sections: _Optional[_Iterable[_Union[CheckReport.SeveritySection, _Mapping]]] = ...) -> None: ...

class ReviewManifest(_message.Message):
    __slots__ = ("name", "areas")
    NAME_FIELD_NUMBER: _ClassVar[int]
    AREAS_FIELD_NUMBER: _ClassVar[int]
    name: str
    areas: _containers.RepeatedCompositeFieldContainer[ManifestArea]
    def __init__(self, name: _Optional[str] = ..., areas: _Optional[_Iterable[_Union[ManifestArea, _Mapping]]] = ...) -> None: ...

class ManifestArea(_message.Message):
    __slots__ = ("name", "items")
    NAME_FIELD_NUMBER: _ClassVar[int]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    name: str
    items: _containers.RepeatedCompositeFieldContainer[ManifestItem]
    def __init__(self, name: _Optional[str] = ..., items: _Optional[_Iterable[_Union[ManifestItem, _Mapping]]] = ...) -> None: ...

class ManifestItem(_message.Message):
    __slots__ = ("id", "title", "description", "note", "binding")
    ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    BINDING_FIELD_NUMBER: _ClassVar[int]
    id: str
    title: str
    description: str
    note: str
    binding: ItemBinding
    def __init__(self, id: _Optional[str] = ..., title: _Optional[str] = ..., description: _Optional[str] = ..., note: _Optional[str] = ..., binding: _Optional[_Union[ItemBinding, _Mapping]] = ...) -> None: ...

class ItemBinding(_message.Message):
    __slots__ = ("rule", "tag", "profile", "query", "present", "scope", "requirement", "applies_to_class")
    RULE_FIELD_NUMBER: _ClassVar[int]
    TAG_FIELD_NUMBER: _ClassVar[int]
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    PRESENT_FIELD_NUMBER: _ClassVar[int]
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    REQUIREMENT_FIELD_NUMBER: _ClassVar[int]
    APPLIES_TO_CLASS_FIELD_NUMBER: _ClassVar[int]
    rule: str
    tag: str
    profile: str
    query: ManifestQuery
    present: ManifestPresent
    scope: ManifestScope
    requirement: str
    applies_to_class: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, rule: _Optional[str] = ..., tag: _Optional[str] = ..., profile: _Optional[str] = ..., query: _Optional[_Union[ManifestQuery, _Mapping]] = ..., present: _Optional[_Union[ManifestPresent, _Mapping]] = ..., scope: _Optional[_Union[ManifestScope, _Mapping]] = ..., requirement: _Optional[str] = ..., applies_to_class: _Optional[_Iterable[str]] = ...) -> None: ...

class ManifestQuery(_message.Message):
    __slots__ = ("match", "subject", "kind", "message", "severity", "param_symbol")
    MATCH_FIELD_NUMBER: _ClassVar[int]
    SUBJECT_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    SEVERITY_FIELD_NUMBER: _ClassVar[int]
    PARAM_SYMBOL_FIELD_NUMBER: _ClassVar[int]
    match: str
    subject: str
    kind: str
    message: str
    severity: str
    param_symbol: str
    def __init__(self, match: _Optional[str] = ..., subject: _Optional[str] = ..., kind: _Optional[str] = ..., message: _Optional[str] = ..., severity: _Optional[str] = ..., param_symbol: _Optional[str] = ...) -> None: ...

class ManifestPresent(_message.Message):
    __slots__ = ()
    CLASS_FIELD_NUMBER: _ClassVar[int]
    def __init__(self, **kwargs) -> None: ...

class ManifestScope(_message.Message):
    __slots__ = ("profiles",)
    PROFILES_FIELD_NUMBER: _ClassVar[int]
    profiles: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, profiles: _Optional[_Iterable[str]] = ...) -> None: ...

class ReviewItem(_message.Message):
    __slots__ = ("id", "title", "outcome", "note", "findings", "unmet")
    ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    OUTCOME_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    FINDINGS_FIELD_NUMBER: _ClassVar[int]
    UNMET_FIELD_NUMBER: _ClassVar[int]
    id: str
    title: str
    outcome: str
    note: str
    findings: _containers.RepeatedCompositeFieldContainer[Finding]
    unmet: _containers.RepeatedCompositeFieldContainer[UnmetDependency]
    def __init__(self, id: _Optional[str] = ..., title: _Optional[str] = ..., outcome: _Optional[str] = ..., note: _Optional[str] = ..., findings: _Optional[_Iterable[_Union[Finding, _Mapping]]] = ..., unmet: _Optional[_Iterable[_Union[UnmetDependency, _Mapping]]] = ...) -> None: ...

class WorkItem(_message.Message):
    __slots__ = ("dependency", "blocked")
    DEPENDENCY_FIELD_NUMBER: _ClassVar[int]
    BLOCKED_FIELD_NUMBER: _ClassVar[int]
    dependency: UnmetDependency
    blocked: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, dependency: _Optional[_Union[UnmetDependency, _Mapping]] = ..., blocked: _Optional[_Iterable[str]] = ...) -> None: ...

class UnmetDependency(_message.Message):
    __slots__ = ("mpn", "manufacturer", "symbol", "spec_absent")
    MPN_FIELD_NUMBER: _ClassVar[int]
    MANUFACTURER_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    SPEC_ABSENT_FIELD_NUMBER: _ClassVar[int]
    mpn: str
    manufacturer: str
    symbol: str
    spec_absent: bool
    def __init__(self, mpn: _Optional[str] = ..., manufacturer: _Optional[str] = ..., symbol: _Optional[str] = ..., spec_absent: _Optional[bool] = ...) -> None: ...

class ReviewArea(_message.Message):
    __slots__ = ("name", "items")
    NAME_FIELD_NUMBER: _ClassVar[int]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    name: str
    items: _containers.RepeatedCompositeFieldContainer[ReviewItem]
    def __init__(self, name: _Optional[str] = ..., items: _Optional[_Iterable[_Union[ReviewItem, _Mapping]]] = ...) -> None: ...

class CheckResults(_message.Message):
    __slots__ = ("meta", "design", "run", "catalog", "skipped", "findings", "manifest", "areas", "import_summary", "manifest_snapshot")
    META_FIELD_NUMBER: _ClassVar[int]
    DESIGN_FIELD_NUMBER: _ClassVar[int]
    RUN_FIELD_NUMBER: _ClassVar[int]
    CATALOG_FIELD_NUMBER: _ClassVar[int]
    SKIPPED_FIELD_NUMBER: _ClassVar[int]
    FINDINGS_FIELD_NUMBER: _ClassVar[int]
    MANIFEST_FIELD_NUMBER: _ClassVar[int]
    AREAS_FIELD_NUMBER: _ClassVar[int]
    IMPORT_SUMMARY_FIELD_NUMBER: _ClassVar[int]
    MANIFEST_SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    meta: ResultsMeta
    design: DesignRef
    run: RunConfig
    catalog: _containers.RepeatedCompositeFieldContainer[RuleRecord]
    skipped: _containers.RepeatedCompositeFieldContainer[SkippedRule]
    findings: _containers.RepeatedCompositeFieldContainer[Finding]
    manifest: str
    areas: _containers.RepeatedCompositeFieldContainer[ReviewArea]
    import_summary: ImportSummary
    manifest_snapshot: ReviewManifest
    def __init__(self, meta: _Optional[_Union[ResultsMeta, _Mapping]] = ..., design: _Optional[_Union[DesignRef, _Mapping]] = ..., run: _Optional[_Union[RunConfig, _Mapping]] = ..., catalog: _Optional[_Iterable[_Union[RuleRecord, _Mapping]]] = ..., skipped: _Optional[_Iterable[_Union[SkippedRule, _Mapping]]] = ..., findings: _Optional[_Iterable[_Union[Finding, _Mapping]]] = ..., manifest: _Optional[str] = ..., areas: _Optional[_Iterable[_Union[ReviewArea, _Mapping]]] = ..., import_summary: _Optional[_Union[ImportSummary, _Mapping]] = ..., manifest_snapshot: _Optional[_Union[ReviewManifest, _Mapping]] = ...) -> None: ...

class ImportSummary(_message.Message):
    __slots__ = ("findings", "joined", "unjoined")
    FINDINGS_FIELD_NUMBER: _ClassVar[int]
    JOINED_FIELD_NUMBER: _ClassVar[int]
    UNJOINED_FIELD_NUMBER: _ClassVar[int]
    findings: int
    joined: int
    unjoined: _containers.RepeatedCompositeFieldContainer[UnjoinedReason]
    def __init__(self, findings: _Optional[int] = ..., joined: _Optional[int] = ..., unjoined: _Optional[_Iterable[_Union[UnjoinedReason, _Mapping]]] = ...) -> None: ...

class UnjoinedReason(_message.Message):
    __slots__ = ("reason", "count", "examples")
    REASON_FIELD_NUMBER: _ClassVar[int]
    COUNT_FIELD_NUMBER: _ClassVar[int]
    EXAMPLES_FIELD_NUMBER: _ClassVar[int]
    reason: str
    count: int
    examples: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, reason: _Optional[str] = ..., count: _Optional[int] = ..., examples: _Optional[_Iterable[str]] = ...) -> None: ...

class ResultsMeta(_message.Message):
    __slots__ = ("schema", "producer", "producer_version", "created_at", "coverage_axis")
    SCHEMA_FIELD_NUMBER: _ClassVar[int]
    PRODUCER_FIELD_NUMBER: _ClassVar[int]
    PRODUCER_VERSION_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    COVERAGE_AXIS_FIELD_NUMBER: _ClassVar[int]
    schema: str
    producer: str
    producer_version: str
    created_at: str
    coverage_axis: bool
    def __init__(self, schema: _Optional[str] = ..., producer: _Optional[str] = ..., producer_version: _Optional[str] = ..., created_at: _Optional[str] = ..., coverage_axis: _Optional[bool] = ...) -> None: ...

class DesignRef(_message.Message):
    __slots__ = ("source", "content_hash")
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    source: str
    content_hash: str
    def __init__(self, source: _Optional[str] = ..., content_hash: _Optional[str] = ...) -> None: ...

class RunConfig(_message.Message):
    __slots__ = ("params", "profiles", "intent", "conventions", "ratified_floor")
    PARAMS_FIELD_NUMBER: _ClassVar[int]
    PROFILES_FIELD_NUMBER: _ClassVar[int]
    INTENT_FIELD_NUMBER: _ClassVar[int]
    CONVENTIONS_FIELD_NUMBER: _ClassVar[int]
    RATIFIED_FLOOR_FIELD_NUMBER: _ClassVar[int]
    params: bool
    profiles: bool
    intent: bool
    conventions: str
    ratified_floor: float
    def __init__(self, params: _Optional[bool] = ..., profiles: _Optional[bool] = ..., intent: _Optional[bool] = ..., conventions: _Optional[str] = ..., ratified_floor: _Optional[float] = ...) -> None: ...

class SkippedRule(_message.Message):
    __slots__ = ("name", "reason")
    NAME_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    name: str
    reason: str
    def __init__(self, name: _Optional[str] = ..., reason: _Optional[str] = ...) -> None: ...

class RuleRecord(_message.Message):
    __slots__ = ("name", "severity", "summary", "tags")
    class TagsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    SEVERITY_FIELD_NUMBER: _ClassVar[int]
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    name: str
    severity: str
    summary: str
    tags: _containers.ScalarMap[str, str]
    def __init__(self, name: _Optional[str] = ..., severity: _Optional[str] = ..., summary: _Optional[str] = ..., tags: _Optional[_Mapping[str, str]] = ...) -> None: ...
