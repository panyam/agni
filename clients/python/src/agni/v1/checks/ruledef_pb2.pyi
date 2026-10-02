from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RuleDeck(_message.Message):
    __slots__ = ("name", "source", "rules")
    NAME_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    RULES_FIELD_NUMBER: _ClassVar[int]
    name: str
    source: str
    rules: _containers.RepeatedCompositeFieldContainer[RuleDef]
    def __init__(self, name: _Optional[str] = ..., source: _Optional[str] = ..., rules: _Optional[_Iterable[_Union[RuleDef, _Mapping]]] = ...) -> None: ...

class RuleDef(_message.Message):
    __slots__ = ("spec", "query", "profile")
    SPEC_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    spec: SpecRule
    query: QueryRule
    profile: ProfileDef
    def __init__(self, spec: _Optional[_Union[SpecRule, _Mapping]] = ..., query: _Optional[_Union[QueryRule, _Mapping]] = ..., profile: _Optional[_Union[ProfileDef, _Mapping]] = ...) -> None: ...

class RuleMeta(_message.Message):
    __slots__ = ("name", "severity", "summary", "impact", "detail", "tags", "optional_reads", "requires_capability", "remedy")
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
    IMPACT_FIELD_NUMBER: _ClassVar[int]
    DETAIL_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    OPTIONAL_READS_FIELD_NUMBER: _ClassVar[int]
    REQUIRES_CAPABILITY_FIELD_NUMBER: _ClassVar[int]
    REMEDY_FIELD_NUMBER: _ClassVar[int]
    name: str
    severity: str
    summary: str
    impact: str
    detail: str
    tags: _containers.ScalarMap[str, str]
    optional_reads: _containers.RepeatedScalarFieldContainer[str]
    requires_capability: _containers.RepeatedScalarFieldContainer[str]
    remedy: str
    def __init__(self, name: _Optional[str] = ..., severity: _Optional[str] = ..., summary: _Optional[str] = ..., impact: _Optional[str] = ..., detail: _Optional[str] = ..., tags: _Optional[_Mapping[str, str]] = ..., optional_reads: _Optional[_Iterable[str]] = ..., requires_capability: _Optional[_Iterable[str]] = ..., remedy: _Optional[str] = ...) -> None: ...

class SpecRule(_message.Message):
    __slots__ = ("meta", "body")
    META_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    meta: RuleMeta
    body: SpecBody
    def __init__(self, meta: _Optional[_Union[RuleMeta, _Mapping]] = ..., body: _Optional[_Union[SpecBody, _Mapping]] = ...) -> None: ...

class SpecBody(_message.Message):
    __slots__ = ("over", "let", "where", "message", "scope")
    class LetEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: SpecTerm
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[SpecTerm, _Mapping]] = ...) -> None: ...
    OVER_FIELD_NUMBER: _ClassVar[int]
    LET_FIELD_NUMBER: _ClassVar[int]
    WHERE_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    over: str
    let: _containers.MessageMap[str, SpecTerm]
    where: SpecExpr
    message: str
    scope: SpecExpr
    def __init__(self, over: _Optional[str] = ..., let: _Optional[_Mapping[str, SpecTerm]] = ..., where: _Optional[_Union[SpecExpr, _Mapping]] = ..., message: _Optional[str] = ..., scope: _Optional[_Union[SpecExpr, _Mapping]] = ...) -> None: ...

class SpecTerm(_message.Message):
    __slots__ = ("lit", "fact", "var", "call", "count_of")
    LIT_FIELD_NUMBER: _ClassVar[int]
    FACT_FIELD_NUMBER: _ClassVar[int]
    VAR_FIELD_NUMBER: _ClassVar[int]
    CALL_FIELD_NUMBER: _ClassVar[int]
    COUNT_OF_FIELD_NUMBER: _ClassVar[int]
    lit: SpecLit
    fact: str
    var: str
    call: SpecCall
    count_of: SpecCountOf
    def __init__(self, lit: _Optional[_Union[SpecLit, _Mapping]] = ..., fact: _Optional[str] = ..., var: _Optional[str] = ..., call: _Optional[_Union[SpecCall, _Mapping]] = ..., count_of: _Optional[_Union[SpecCountOf, _Mapping]] = ...) -> None: ...

class SpecLit(_message.Message):
    __slots__ = ("s", "i", "b")
    S_FIELD_NUMBER: _ClassVar[int]
    I_FIELD_NUMBER: _ClassVar[int]
    B_FIELD_NUMBER: _ClassVar[int]
    s: str
    i: int
    b: bool
    def __init__(self, s: _Optional[str] = ..., i: _Optional[int] = ..., b: _Optional[bool] = ...) -> None: ...

class SpecCall(_message.Message):
    __slots__ = ("fn", "args")
    FN_FIELD_NUMBER: _ClassVar[int]
    ARGS_FIELD_NUMBER: _ClassVar[int]
    fn: str
    args: _containers.RepeatedCompositeFieldContainer[SpecTerm]
    def __init__(self, fn: _Optional[str] = ..., args: _Optional[_Iterable[_Union[SpecTerm, _Mapping]]] = ...) -> None: ...

class SpecCountOf(_message.Message):
    __slots__ = ("over", "where")
    OVER_FIELD_NUMBER: _ClassVar[int]
    WHERE_FIELD_NUMBER: _ClassVar[int]
    over: str
    where: SpecExpr
    def __init__(self, over: _Optional[str] = ..., where: _Optional[_Union[SpecExpr, _Mapping]] = ...) -> None: ...

class SpecExpr(_message.Message):
    __slots__ = ("cmp", "match", "exists_in", "is_true")
    AND_FIELD_NUMBER: _ClassVar[int]
    OR_FIELD_NUMBER: _ClassVar[int]
    NOT_FIELD_NUMBER: _ClassVar[int]
    CMP_FIELD_NUMBER: _ClassVar[int]
    IN_FIELD_NUMBER: _ClassVar[int]
    MATCH_FIELD_NUMBER: _ClassVar[int]
    EXISTS_IN_FIELD_NUMBER: _ClassVar[int]
    IS_TRUE_FIELD_NUMBER: _ClassVar[int]
    cmp: SpecCmp
    match: SpecMatch
    exists_in: SpecExistsIn
    is_true: SpecTerm
    def __init__(self, cmp: _Optional[_Union[SpecCmp, _Mapping]] = ..., match: _Optional[_Union[SpecMatch, _Mapping]] = ..., exists_in: _Optional[_Union[SpecExistsIn, _Mapping]] = ..., is_true: _Optional[_Union[SpecTerm, _Mapping]] = ..., **kwargs) -> None: ...

class SpecExprList(_message.Message):
    __slots__ = ("xs",)
    XS_FIELD_NUMBER: _ClassVar[int]
    xs: _containers.RepeatedCompositeFieldContainer[SpecExpr]
    def __init__(self, xs: _Optional[_Iterable[_Union[SpecExpr, _Mapping]]] = ...) -> None: ...

class SpecCmp(_message.Message):
    __slots__ = ("l", "op", "r")
    L_FIELD_NUMBER: _ClassVar[int]
    OP_FIELD_NUMBER: _ClassVar[int]
    R_FIELD_NUMBER: _ClassVar[int]
    l: SpecTerm
    op: str
    r: SpecTerm
    def __init__(self, l: _Optional[_Union[SpecTerm, _Mapping]] = ..., op: _Optional[str] = ..., r: _Optional[_Union[SpecTerm, _Mapping]] = ...) -> None: ...

class SpecIn(_message.Message):
    __slots__ = ("t", "set")
    T_FIELD_NUMBER: _ClassVar[int]
    SET_FIELD_NUMBER: _ClassVar[int]
    t: SpecTerm
    set: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, t: _Optional[_Union[SpecTerm, _Mapping]] = ..., set: _Optional[_Iterable[str]] = ...) -> None: ...

class SpecMatch(_message.Message):
    __slots__ = ("t", "pattern")
    T_FIELD_NUMBER: _ClassVar[int]
    PATTERN_FIELD_NUMBER: _ClassVar[int]
    t: SpecTerm
    pattern: str
    def __init__(self, t: _Optional[_Union[SpecTerm, _Mapping]] = ..., pattern: _Optional[str] = ...) -> None: ...

class SpecExistsIn(_message.Message):
    __slots__ = ("over", "where")
    OVER_FIELD_NUMBER: _ClassVar[int]
    WHERE_FIELD_NUMBER: _ClassVar[int]
    over: str
    where: SpecExpr
    def __init__(self, over: _Optional[str] = ..., where: _Optional[_Union[SpecExpr, _Mapping]] = ...) -> None: ...

class QueryRule(_message.Message):
    __slots__ = ("meta", "query", "kind", "subject_var", "pin_var", "message", "param_symbol", "context_vars")
    META_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBJECT_VAR_FIELD_NUMBER: _ClassVar[int]
    PIN_VAR_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    PARAM_SYMBOL_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_VARS_FIELD_NUMBER: _ClassVar[int]
    meta: RuleMeta
    query: DatalogQuery
    kind: str
    subject_var: str
    pin_var: str
    message: str
    param_symbol: str
    context_vars: _containers.RepeatedCompositeFieldContainer[ContextVar]
    def __init__(self, meta: _Optional[_Union[RuleMeta, _Mapping]] = ..., query: _Optional[_Union[DatalogQuery, _Mapping]] = ..., kind: _Optional[str] = ..., subject_var: _Optional[str] = ..., pin_var: _Optional[str] = ..., message: _Optional[str] = ..., param_symbol: _Optional[str] = ..., context_vars: _Optional[_Iterable[_Union[ContextVar, _Mapping]]] = ...) -> None: ...

class ContextVar(_message.Message):
    __slots__ = ("var", "kind", "role")
    VAR_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    var: str
    kind: str
    role: str
    def __init__(self, var: _Optional[str] = ..., kind: _Optional[str] = ..., role: _Optional[str] = ...) -> None: ...

class DatalogQuery(_message.Message):
    __slots__ = ("rules", "goal", "select", "having")
    RULES_FIELD_NUMBER: _ClassVar[int]
    GOAL_FIELD_NUMBER: _ClassVar[int]
    SELECT_FIELD_NUMBER: _ClassVar[int]
    HAVING_FIELD_NUMBER: _ClassVar[int]
    rules: _containers.RepeatedCompositeFieldContainer[DatalogRule]
    goal: DatalogBody
    select: _containers.RepeatedCompositeFieldContainer[DatalogTerm]
    having: _containers.RepeatedCompositeFieldContainer[DatalogCompare]
    def __init__(self, rules: _Optional[_Iterable[_Union[DatalogRule, _Mapping]]] = ..., goal: _Optional[_Union[DatalogBody, _Mapping]] = ..., select: _Optional[_Iterable[_Union[DatalogTerm, _Mapping]]] = ..., having: _Optional[_Iterable[_Union[DatalogCompare, _Mapping]]] = ...) -> None: ...

class DatalogRule(_message.Message):
    __slots__ = ("head", "body", "hops")
    HEAD_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    HOPS_FIELD_NUMBER: _ClassVar[int]
    head: DatalogAtom
    body: DatalogBody
    hops: int
    def __init__(self, head: _Optional[_Union[DatalogAtom, _Mapping]] = ..., body: _Optional[_Union[DatalogBody, _Mapping]] = ..., hops: _Optional[int] = ...) -> None: ...

class DatalogBody(_message.Message):
    __slots__ = ("literals",)
    LITERALS_FIELD_NUMBER: _ClassVar[int]
    literals: _containers.RepeatedCompositeFieldContainer[DatalogLiteral]
    def __init__(self, literals: _Optional[_Iterable[_Union[DatalogLiteral, _Mapping]]] = ...) -> None: ...

class DatalogLiteral(_message.Message):
    __slots__ = ("pos", "neg", "compare")
    POS_FIELD_NUMBER: _ClassVar[int]
    NEG_FIELD_NUMBER: _ClassVar[int]
    COMPARE_FIELD_NUMBER: _ClassVar[int]
    pos: DatalogAtom
    neg: DatalogAtom
    compare: DatalogCompare
    def __init__(self, pos: _Optional[_Union[DatalogAtom, _Mapping]] = ..., neg: _Optional[_Union[DatalogAtom, _Mapping]] = ..., compare: _Optional[_Union[DatalogCompare, _Mapping]] = ...) -> None: ...

class DatalogAtom(_message.Message):
    __slots__ = ("relation", "args")
    RELATION_FIELD_NUMBER: _ClassVar[int]
    ARGS_FIELD_NUMBER: _ClassVar[int]
    relation: str
    args: _containers.RepeatedCompositeFieldContainer[DatalogTerm]
    def __init__(self, relation: _Optional[str] = ..., args: _Optional[_Iterable[_Union[DatalogTerm, _Mapping]]] = ...) -> None: ...

class DatalogCompare(_message.Message):
    __slots__ = ("left", "op", "right")
    LEFT_FIELD_NUMBER: _ClassVar[int]
    OP_FIELD_NUMBER: _ClassVar[int]
    RIGHT_FIELD_NUMBER: _ClassVar[int]
    left: DatalogTerm
    op: str
    right: DatalogTerm
    def __init__(self, left: _Optional[_Union[DatalogTerm, _Mapping]] = ..., op: _Optional[str] = ..., right: _Optional[_Union[DatalogTerm, _Mapping]] = ...) -> None: ...

class DatalogTerm(_message.Message):
    __slots__ = ("var", "constant", "agg")
    VAR_FIELD_NUMBER: _ClassVar[int]
    CONSTANT_FIELD_NUMBER: _ClassVar[int]
    AGG_FIELD_NUMBER: _ClassVar[int]
    var: str
    constant: DatalogValue
    agg: DatalogAggregate
    def __init__(self, var: _Optional[str] = ..., constant: _Optional[_Union[DatalogValue, _Mapping]] = ..., agg: _Optional[_Union[DatalogAggregate, _Mapping]] = ...) -> None: ...

class DatalogValue(_message.Message):
    __slots__ = ("s", "num", "absent", "base_unit")
    S_FIELD_NUMBER: _ClassVar[int]
    NUM_FIELD_NUMBER: _ClassVar[int]
    ABSENT_FIELD_NUMBER: _ClassVar[int]
    BASE_UNIT_FIELD_NUMBER: _ClassVar[int]
    s: str
    num: float
    absent: bool
    base_unit: str
    def __init__(self, s: _Optional[str] = ..., num: _Optional[float] = ..., absent: _Optional[bool] = ..., base_unit: _Optional[str] = ...) -> None: ...

class DatalogAggregate(_message.Message):
    __slots__ = ("func", "var", "distinct")
    FUNC_FIELD_NUMBER: _ClassVar[int]
    VAR_FIELD_NUMBER: _ClassVar[int]
    DISTINCT_FIELD_NUMBER: _ClassVar[int]
    func: str
    var: str
    distinct: bool
    def __init__(self, func: _Optional[str] = ..., var: _Optional[str] = ..., distinct: _Optional[bool] = ...) -> None: ...

class ProfileDef(_message.Message):
    __slots__ = ("name", "signals", "requirements", "host")
    NAME_FIELD_NUMBER: _ClassVar[int]
    SIGNALS_FIELD_NUMBER: _ClassVar[int]
    REQUIREMENTS_FIELD_NUMBER: _ClassVar[int]
    HOST_FIELD_NUMBER: _ClassVar[int]
    name: str
    signals: _containers.RepeatedCompositeFieldContainer[ProfileSignal]
    requirements: _containers.RepeatedCompositeFieldContainer[ProfileRequirement]
    host: ProfileHost
    def __init__(self, name: _Optional[str] = ..., signals: _Optional[_Iterable[_Union[ProfileSignal, _Mapping]]] = ..., requirements: _Optional[_Iterable[_Union[ProfileRequirement, _Mapping]]] = ..., host: _Optional[_Union[ProfileHost, _Mapping]] = ...) -> None: ...

class ProfileHost(_message.Message):
    __slots__ = ("attr", "value")
    ATTR_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    CLASS_FIELD_NUMBER: _ClassVar[int]
    attr: str
    value: str
    def __init__(self, attr: _Optional[str] = ..., value: _Optional[str] = ..., **kwargs) -> None: ...

class ProfileNamingMap(_message.Message):
    __slots__ = ("override", "suffixes")
    class SuffixesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    OVERRIDE_FIELD_NUMBER: _ClassVar[int]
    SUFFIXES_FIELD_NUMBER: _ClassVar[int]
    override: str
    suffixes: _containers.ScalarMap[str, str]
    def __init__(self, override: _Optional[str] = ..., suffixes: _Optional[_Mapping[str, str]] = ...) -> None: ...

class ProfileSignal(_message.Message):
    __slots__ = ("name", "prefix", "suffix", "glob", "regex", "pullup", "anchor")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PREFIX_FIELD_NUMBER: _ClassVar[int]
    SUFFIX_FIELD_NUMBER: _ClassVar[int]
    GLOB_FIELD_NUMBER: _ClassVar[int]
    REGEX_FIELD_NUMBER: _ClassVar[int]
    PULLUP_FIELD_NUMBER: _ClassVar[int]
    ANCHOR_FIELD_NUMBER: _ClassVar[int]
    name: str
    prefix: str
    suffix: str
    glob: str
    regex: str
    pullup: bool
    anchor: bool
    def __init__(self, name: _Optional[str] = ..., prefix: _Optional[str] = ..., suffix: _Optional[str] = ..., glob: _Optional[str] = ..., regex: _Optional[str] = ..., pullup: _Optional[bool] = ..., anchor: _Optional[bool] = ...) -> None: ...

class ProfileRequirement(_message.Message):
    __slots__ = ("type", "params")
    class ParamsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    TYPE_FIELD_NUMBER: _ClassVar[int]
    PARAMS_FIELD_NUMBER: _ClassVar[int]
    type: str
    params: _containers.ScalarMap[str, str]
    def __init__(self, type: _Optional[str] = ..., params: _Optional[_Mapping[str, str]] = ...) -> None: ...
