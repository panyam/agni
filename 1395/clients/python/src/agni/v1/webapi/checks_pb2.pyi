from agni.v1.checks import checks_pb2 as _checks_pb2
from agni.v1.config import intent_pb2 as _intent_pb2
from agni.v1.config import naming_pb2 as _naming_pb2
from agni.v1.ir import ir_pb2 as _ir_pb2
from agni.v1.webapi import config_pb2 as _config_pb2
from agni.v1.webapi import query_pb2 as _query_pb2
from agni.v1.param import param_pb2 as _param_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class CheckReportFormat(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CHECK_REPORT_FORMAT_UNSPECIFIED: _ClassVar[CheckReportFormat]
    CHECK_REPORT_FORMAT_RESULTS_JSON: _ClassVar[CheckReportFormat]
    CHECK_REPORT_FORMAT_HTML: _ClassVar[CheckReportFormat]
CHECK_REPORT_FORMAT_UNSPECIFIED: CheckReportFormat
CHECK_REPORT_FORMAT_RESULTS_JSON: CheckReportFormat
CHECK_REPORT_FORMAT_HTML: CheckReportFormat

class CheckDesignRequest(_message.Message):
    __slots__ = ("snapshot", "rules", "work_budget", "subjects")
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    RULES_FIELD_NUMBER: _ClassVar[int]
    WORK_BUDGET_FIELD_NUMBER: _ClassVar[int]
    SUBJECTS_FIELD_NUMBER: _ClassVar[int]
    snapshot: str
    rules: _containers.RepeatedScalarFieldContainer[str]
    work_budget: int
    subjects: _containers.RepeatedCompositeFieldContainer[_checks_pb2.Subject]
    def __init__(self, snapshot: _Optional[str] = ..., rules: _Optional[_Iterable[str]] = ..., work_budget: _Optional[int] = ..., subjects: _Optional[_Iterable[_Union[_checks_pb2.Subject, _Mapping]]] = ...) -> None: ...

class CheckDesignResponse(_message.Message):
    __slots__ = ("findings", "skipped", "verdicts", "unexpanded_hierarchy", "io_map")
    FINDINGS_FIELD_NUMBER: _ClassVar[int]
    SKIPPED_FIELD_NUMBER: _ClassVar[int]
    VERDICTS_FIELD_NUMBER: _ClassVar[int]
    UNEXPANDED_HIERARCHY_FIELD_NUMBER: _ClassVar[int]
    IO_MAP_FIELD_NUMBER: _ClassVar[int]
    findings: _containers.RepeatedCompositeFieldContainer[_checks_pb2.Finding]
    skipped: _containers.RepeatedCompositeFieldContainer[SkippedRule]
    verdicts: _containers.RepeatedCompositeFieldContainer[_checks_pb2.Verdict]
    unexpanded_hierarchy: _containers.RepeatedCompositeFieldContainer[_ir_pb2.UnexpandedHierarchy]
    io_map: _containers.RepeatedCompositeFieldContainer[_intent_pb2.PinAssignment]
    def __init__(self, findings: _Optional[_Iterable[_Union[_checks_pb2.Finding, _Mapping]]] = ..., skipped: _Optional[_Iterable[_Union[SkippedRule, _Mapping]]] = ..., verdicts: _Optional[_Iterable[_Union[_checks_pb2.Verdict, _Mapping]]] = ..., unexpanded_hierarchy: _Optional[_Iterable[_Union[_ir_pb2.UnexpandedHierarchy, _Mapping]]] = ..., io_map: _Optional[_Iterable[_Union[_intent_pb2.PinAssignment, _Mapping]]] = ...) -> None: ...

class SkippedRule(_message.Message):
    __slots__ = ("name", "reason", "design_wide")
    NAME_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    DESIGN_WIDE_FIELD_NUMBER: _ClassVar[int]
    name: str
    reason: str
    design_wide: bool
    def __init__(self, name: _Optional[str] = ..., reason: _Optional[str] = ..., design_wide: _Optional[bool] = ...) -> None: ...

class GetCheckReportRequest(_message.Message):
    __slots__ = ("snapshot", "rules")
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    RULES_FIELD_NUMBER: _ClassVar[int]
    snapshot: str
    rules: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, snapshot: _Optional[str] = ..., rules: _Optional[_Iterable[str]] = ...) -> None: ...

class GetCheckReportResponse(_message.Message):
    __slots__ = ("report",)
    REPORT_FIELD_NUMBER: _ClassVar[int]
    report: _checks_pb2.CheckReport
    def __init__(self, report: _Optional[_Union[_checks_pb2.CheckReport, _Mapping]] = ...) -> None: ...

class ReportLinks(_message.Message):
    __slots__ = ("url_base", "design_uri", "withheld")
    URL_BASE_FIELD_NUMBER: _ClassVar[int]
    DESIGN_URI_FIELD_NUMBER: _ClassVar[int]
    WITHHELD_FIELD_NUMBER: _ClassVar[int]
    url_base: str
    design_uri: str
    withheld: str
    def __init__(self, url_base: _Optional[str] = ..., design_uri: _Optional[str] = ..., withheld: _Optional[str] = ...) -> None: ...

class RenderCheckReportRequest(_message.Message):
    __slots__ = ("snapshot", "rules", "run", "format", "links", "query_sets")
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    RULES_FIELD_NUMBER: _ClassVar[int]
    RUN_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    LINKS_FIELD_NUMBER: _ClassVar[int]
    QUERY_SETS_FIELD_NUMBER: _ClassVar[int]
    snapshot: str
    rules: _containers.RepeatedScalarFieldContainer[str]
    run: CheckDesignResponse
    format: CheckReportFormat
    links: ReportLinks
    query_sets: _containers.RepeatedCompositeFieldContainer[_query_pb2.RunQueriesResponse]
    def __init__(self, snapshot: _Optional[str] = ..., rules: _Optional[_Iterable[str]] = ..., run: _Optional[_Union[CheckDesignResponse, _Mapping]] = ..., format: _Optional[_Union[CheckReportFormat, str]] = ..., links: _Optional[_Union[ReportLinks, _Mapping]] = ..., query_sets: _Optional[_Iterable[_Union[_query_pb2.RunQueriesResponse, _Mapping]]] = ...) -> None: ...

class RenderCheckReportResponse(_message.Message):
    __slots__ = ("content", "content_type", "filename")
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    CONTENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    FILENAME_FIELD_NUMBER: _ClassVar[int]
    content: bytes
    content_type: str
    filename: str
    def __init__(self, content: _Optional[bytes] = ..., content_type: _Optional[str] = ..., filename: _Optional[str] = ...) -> None: ...

class GetNamingConventionRequest(_message.Message):
    __slots__ = ("uri",)
    URI_FIELD_NUMBER: _ClassVar[int]
    uri: str
    def __init__(self, uri: _Optional[str] = ...) -> None: ...

class GetNamingConventionResponse(_message.Message):
    __slots__ = ("convention",)
    CONVENTION_FIELD_NUMBER: _ClassVar[int]
    convention: _naming_pb2.NamingConvention
    def __init__(self, convention: _Optional[_Union[_naming_pb2.NamingConvention, _Mapping]] = ...) -> None: ...

class GetExpectationsRequest(_message.Message):
    __slots__ = ("snapshot",)
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    snapshot: str
    def __init__(self, snapshot: _Optional[str] = ...) -> None: ...

class RuleExpectation(_message.Message):
    __slots__ = ("rule", "subjects", "pending", "why")
    RULE_FIELD_NUMBER: _ClassVar[int]
    SUBJECTS_FIELD_NUMBER: _ClassVar[int]
    PENDING_FIELD_NUMBER: _ClassVar[int]
    WHY_FIELD_NUMBER: _ClassVar[int]
    rule: str
    subjects: _containers.RepeatedScalarFieldContainer[str]
    pending: bool
    why: str
    def __init__(self, rule: _Optional[str] = ..., subjects: _Optional[_Iterable[str]] = ..., pending: _Optional[bool] = ..., why: _Optional[str] = ...) -> None: ...

class GetExpectationsResponse(_message.Message):
    __slots__ = ("expectations", "has_sidecar")
    EXPECTATIONS_FIELD_NUMBER: _ClassVar[int]
    HAS_SIDECAR_FIELD_NUMBER: _ClassVar[int]
    expectations: _containers.RepeatedCompositeFieldContainer[RuleExpectation]
    has_sidecar: bool
    def __init__(self, expectations: _Optional[_Iterable[_Union[RuleExpectation, _Mapping]]] = ..., has_sidecar: _Optional[bool] = ...) -> None: ...

class ListRulesRequest(_message.Message):
    __slots__ = ("snapshot",)
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    snapshot: str
    def __init__(self, snapshot: _Optional[str] = ...) -> None: ...

class RuleInfo(_message.Message):
    __slots__ = ("name", "severity", "summary", "reads", "tags", "available", "unavailable_reason", "impact", "detail", "remedy", "shipped", "doc_url")
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
    READS_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    UNAVAILABLE_REASON_FIELD_NUMBER: _ClassVar[int]
    IMPACT_FIELD_NUMBER: _ClassVar[int]
    DETAIL_FIELD_NUMBER: _ClassVar[int]
    REMEDY_FIELD_NUMBER: _ClassVar[int]
    SHIPPED_FIELD_NUMBER: _ClassVar[int]
    DOC_URL_FIELD_NUMBER: _ClassVar[int]
    name: str
    severity: str
    summary: str
    reads: _containers.RepeatedScalarFieldContainer[str]
    tags: _containers.ScalarMap[str, str]
    available: bool
    unavailable_reason: str
    impact: str
    detail: str
    remedy: str
    shipped: bool
    doc_url: str
    def __init__(self, name: _Optional[str] = ..., severity: _Optional[str] = ..., summary: _Optional[str] = ..., reads: _Optional[_Iterable[str]] = ..., tags: _Optional[_Mapping[str, str]] = ..., available: _Optional[bool] = ..., unavailable_reason: _Optional[str] = ..., impact: _Optional[str] = ..., detail: _Optional[str] = ..., remedy: _Optional[str] = ..., shipped: _Optional[bool] = ..., doc_url: _Optional[str] = ...) -> None: ...

class ListRulesResponse(_message.Message):
    __slots__ = ("rules", "engine_version")
    RULES_FIELD_NUMBER: _ClassVar[int]
    ENGINE_VERSION_FIELD_NUMBER: _ClassVar[int]
    rules: _containers.RepeatedCompositeFieldContainer[RuleInfo]
    engine_version: str
    def __init__(self, rules: _Optional[_Iterable[_Union[RuleInfo, _Mapping]]] = ..., engine_version: _Optional[str] = ...) -> None: ...

class GetInterfaceCoverageRequest(_message.Message):
    __slots__ = ("snapshot",)
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    snapshot: str
    def __init__(self, snapshot: _Optional[str] = ...) -> None: ...

class SignalCoverage(_message.Message):
    __slots__ = ("name", "net", "state")
    NAME_FIELD_NUMBER: _ClassVar[int]
    NET_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    name: str
    net: str
    state: str
    def __init__(self, name: _Optional[str] = ..., net: _Optional[str] = ..., state: _Optional[str] = ...) -> None: ...

class InterfaceCoverage(_message.Message):
    __slots__ = ("profile", "anchor_net", "signals")
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    ANCHOR_NET_FIELD_NUMBER: _ClassVar[int]
    SIGNALS_FIELD_NUMBER: _ClassVar[int]
    profile: str
    anchor_net: str
    signals: _containers.RepeatedCompositeFieldContainer[SignalCoverage]
    def __init__(self, profile: _Optional[str] = ..., anchor_net: _Optional[str] = ..., signals: _Optional[_Iterable[_Union[SignalCoverage, _Mapping]]] = ...) -> None: ...

class GetInterfaceCoverageResponse(_message.Message):
    __slots__ = ("interfaces",)
    INTERFACES_FIELD_NUMBER: _ClassVar[int]
    interfaces: _containers.RepeatedCompositeFieldContainer[InterfaceCoverage]
    def __init__(self, interfaces: _Optional[_Iterable[_Union[InterfaceCoverage, _Mapping]]] = ...) -> None: ...

class GetComponentParamsRequest(_message.Message):
    __slots__ = ("snapshot",)
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    snapshot: str
    def __init__(self, snapshot: _Optional[str] = ...) -> None: ...

class ComponentParams(_message.Message):
    __slots__ = ("ref_des", "mpn", "spec", "corpus")
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    MPN_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    CORPUS_FIELD_NUMBER: _ClassVar[int]
    ref_des: str
    mpn: str
    spec: _param_pb2.PartSpec
    corpus: str
    def __init__(self, ref_des: _Optional[str] = ..., mpn: _Optional[str] = ..., spec: _Optional[_Union[_param_pb2.PartSpec, _Mapping]] = ..., corpus: _Optional[str] = ...) -> None: ...

class GetComponentParamsResponse(_message.Message):
    __slots__ = ("components", "corpus_connected")
    COMPONENTS_FIELD_NUMBER: _ClassVar[int]
    CORPUS_CONNECTED_FIELD_NUMBER: _ClassVar[int]
    components: _containers.RepeatedCompositeFieldContainer[ComponentParams]
    corpus_connected: bool
    def __init__(self, components: _Optional[_Iterable[_Union[ComponentParams, _Mapping]]] = ..., corpus_connected: _Optional[bool] = ...) -> None: ...
