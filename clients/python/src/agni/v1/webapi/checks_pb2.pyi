from agni.v1.checks import checks_pb2 as _checks_pb2
from agni.v1.config import naming_pb2 as _naming_pb2
from agni.v1.webapi import config_pb2 as _config_pb2
from agni.v1.param import param_pb2 as _param_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class CheckDesignRequest(_message.Message):
    __slots__ = ("rules", "overlay", "uri", "board_uri", "as_named", "work_budget")
    RULES_FIELD_NUMBER: _ClassVar[int]
    OVERLAY_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    BOARD_URI_FIELD_NUMBER: _ClassVar[int]
    AS_NAMED_FIELD_NUMBER: _ClassVar[int]
    WORK_BUDGET_FIELD_NUMBER: _ClassVar[int]
    rules: _containers.RepeatedScalarFieldContainer[str]
    overlay: OverlayConfig
    uri: str
    board_uri: str
    as_named: bool
    work_budget: int
    def __init__(self, rules: _Optional[_Iterable[str]] = ..., overlay: _Optional[_Union[OverlayConfig, _Mapping]] = ..., uri: _Optional[str] = ..., board_uri: _Optional[str] = ..., as_named: _Optional[bool] = ..., work_budget: _Optional[int] = ...) -> None: ...

class OverlayConfig(_message.Message):
    __slots__ = ("config", "ignore_project")
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    IGNORE_PROJECT_FIELD_NUMBER: _ClassVar[int]
    config: _config_pb2.AnalysisConfig
    ignore_project: bool
    def __init__(self, config: _Optional[_Union[_config_pb2.AnalysisConfig, _Mapping]] = ..., ignore_project: _Optional[bool] = ...) -> None: ...

class CheckDesignResponse(_message.Message):
    __slots__ = ("findings", "skipped", "verdicts")
    FINDINGS_FIELD_NUMBER: _ClassVar[int]
    SKIPPED_FIELD_NUMBER: _ClassVar[int]
    VERDICTS_FIELD_NUMBER: _ClassVar[int]
    findings: _containers.RepeatedCompositeFieldContainer[_checks_pb2.Finding]
    skipped: _containers.RepeatedCompositeFieldContainer[SkippedRule]
    verdicts: _containers.RepeatedCompositeFieldContainer[_checks_pb2.Verdict]
    def __init__(self, findings: _Optional[_Iterable[_Union[_checks_pb2.Finding, _Mapping]]] = ..., skipped: _Optional[_Iterable[_Union[SkippedRule, _Mapping]]] = ..., verdicts: _Optional[_Iterable[_Union[_checks_pb2.Verdict, _Mapping]]] = ...) -> None: ...

class SkippedRule(_message.Message):
    __slots__ = ("name", "reason")
    NAME_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    name: str
    reason: str
    def __init__(self, name: _Optional[str] = ..., reason: _Optional[str] = ...) -> None: ...

class GetCheckReportRequest(_message.Message):
    __slots__ = ("rules", "overlay", "uri", "board_uri", "as_named")
    RULES_FIELD_NUMBER: _ClassVar[int]
    OVERLAY_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    BOARD_URI_FIELD_NUMBER: _ClassVar[int]
    AS_NAMED_FIELD_NUMBER: _ClassVar[int]
    rules: _containers.RepeatedScalarFieldContainer[str]
    overlay: OverlayConfig
    uri: str
    board_uri: str
    as_named: bool
    def __init__(self, rules: _Optional[_Iterable[str]] = ..., overlay: _Optional[_Union[OverlayConfig, _Mapping]] = ..., uri: _Optional[str] = ..., board_uri: _Optional[str] = ..., as_named: _Optional[bool] = ...) -> None: ...

class GetCheckReportResponse(_message.Message):
    __slots__ = ("report",)
    REPORT_FIELD_NUMBER: _ClassVar[int]
    report: _checks_pb2.CheckReport
    def __init__(self, report: _Optional[_Union[_checks_pb2.CheckReport, _Mapping]] = ...) -> None: ...

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
    __slots__ = ("uri",)
    URI_FIELD_NUMBER: _ClassVar[int]
    uri: str
    def __init__(self, uri: _Optional[str] = ...) -> None: ...

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
    __slots__ = ("overlay", "uri")
    OVERLAY_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    overlay: OverlayConfig
    uri: str
    def __init__(self, overlay: _Optional[_Union[OverlayConfig, _Mapping]] = ..., uri: _Optional[str] = ...) -> None: ...

class RuleInfo(_message.Message):
    __slots__ = ("name", "severity", "summary", "reads", "tags", "available", "unavailable_reason", "impact", "detail", "remedy")
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
    def __init__(self, name: _Optional[str] = ..., severity: _Optional[str] = ..., summary: _Optional[str] = ..., reads: _Optional[_Iterable[str]] = ..., tags: _Optional[_Mapping[str, str]] = ..., available: _Optional[bool] = ..., unavailable_reason: _Optional[str] = ..., impact: _Optional[str] = ..., detail: _Optional[str] = ..., remedy: _Optional[str] = ...) -> None: ...

class ListRulesResponse(_message.Message):
    __slots__ = ("rules",)
    RULES_FIELD_NUMBER: _ClassVar[int]
    rules: _containers.RepeatedCompositeFieldContainer[RuleInfo]
    def __init__(self, rules: _Optional[_Iterable[_Union[RuleInfo, _Mapping]]] = ...) -> None: ...

class GetInterfaceCoverageRequest(_message.Message):
    __slots__ = ("uri",)
    URI_FIELD_NUMBER: _ClassVar[int]
    uri: str
    def __init__(self, uri: _Optional[str] = ...) -> None: ...

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
    __slots__ = ("uri",)
    URI_FIELD_NUMBER: _ClassVar[int]
    uri: str
    def __init__(self, uri: _Optional[str] = ...) -> None: ...

class ComponentParams(_message.Message):
    __slots__ = ("ref_des", "mpn", "spec")
    REF_DES_FIELD_NUMBER: _ClassVar[int]
    MPN_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    ref_des: str
    mpn: str
    spec: _param_pb2.PartSpec
    def __init__(self, ref_des: _Optional[str] = ..., mpn: _Optional[str] = ..., spec: _Optional[_Union[_param_pb2.PartSpec, _Mapping]] = ...) -> None: ...

class GetComponentParamsResponse(_message.Message):
    __slots__ = ("components",)
    COMPONENTS_FIELD_NUMBER: _ClassVar[int]
    components: _containers.RepeatedCompositeFieldContainer[ComponentParams]
    def __init__(self, components: _Optional[_Iterable[_Union[ComponentParams, _Mapping]]] = ...) -> None: ...
