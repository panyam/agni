from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class PinFunction(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PIN_FUNCTION_UNSPECIFIED: _ClassVar[PinFunction]
    PIN_FUNCTION_POWER_INPUT: _ClassVar[PinFunction]
    PIN_FUNCTION_POWER_OUTPUT: _ClassVar[PinFunction]
    PIN_FUNCTION_GROUND: _ClassVar[PinFunction]
    PIN_FUNCTION_INPUT: _ClassVar[PinFunction]
    PIN_FUNCTION_OUTPUT: _ClassVar[PinFunction]
    PIN_FUNCTION_BIDIRECTIONAL: _ClassVar[PinFunction]
    PIN_FUNCTION_PASSIVE: _ClassVar[PinFunction]
    PIN_FUNCTION_NO_CONNECT: _ClassVar[PinFunction]

class PinRelationKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PIN_RELATION_KIND_UNSPECIFIED: _ClassVar[PinRelationKind]
    PIN_RELATION_KIND_TRACKING: _ClassVar[PinRelationKind]

class Modality(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MODALITY_UNSPECIFIED: _ClassVar[Modality]
    MODALITY_REQUIRED: _ClassVar[Modality]
    MODALITY_RECOMMENDED: _ClassVar[Modality]

class LimitKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    LIMIT_KIND_UNSPECIFIED: _ClassVar[LimitKind]
    LIMIT_KIND_ABSOLUTE_MAX: _ClassVar[LimitKind]
    LIMIT_KIND_RECOMMENDED_OPERATING: _ClassVar[LimitKind]
    LIMIT_KIND_CHARACTERISTIC: _ClassVar[LimitKind]

class ConditionCoverage(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CONDITION_COVERAGE_UNSPECIFIED: _ClassVar[ConditionCoverage]
    CONDITION_COVERAGE_COMPLETE: _ClassVar[ConditionCoverage]
    CONDITION_COVERAGE_PARTIAL: _ClassVar[ConditionCoverage]
    CONDITION_COVERAGE_UNCONDITIONAL: _ClassVar[ConditionCoverage]
PIN_FUNCTION_UNSPECIFIED: PinFunction
PIN_FUNCTION_POWER_INPUT: PinFunction
PIN_FUNCTION_POWER_OUTPUT: PinFunction
PIN_FUNCTION_GROUND: PinFunction
PIN_FUNCTION_INPUT: PinFunction
PIN_FUNCTION_OUTPUT: PinFunction
PIN_FUNCTION_BIDIRECTIONAL: PinFunction
PIN_FUNCTION_PASSIVE: PinFunction
PIN_FUNCTION_NO_CONNECT: PinFunction
PIN_RELATION_KIND_UNSPECIFIED: PinRelationKind
PIN_RELATION_KIND_TRACKING: PinRelationKind
MODALITY_UNSPECIFIED: Modality
MODALITY_REQUIRED: Modality
MODALITY_RECOMMENDED: Modality
LIMIT_KIND_UNSPECIFIED: LimitKind
LIMIT_KIND_ABSOLUTE_MAX: LimitKind
LIMIT_KIND_RECOMMENDED_OPERATING: LimitKind
LIMIT_KIND_CHARACTERISTIC: LimitKind
CONDITION_COVERAGE_UNSPECIFIED: ConditionCoverage
CONDITION_COVERAGE_COMPLETE: ConditionCoverage
CONDITION_COVERAGE_PARTIAL: ConditionCoverage
CONDITION_COVERAGE_UNCONDITIONAL: ConditionCoverage

class PartSpec(_message.Message):
    __slots__ = ("mpn", "manufacturer", "device_class", "docs", "parameters", "packages", "pins", "relations", "attributes")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    MPN_FIELD_NUMBER: _ClassVar[int]
    MANUFACTURER_FIELD_NUMBER: _ClassVar[int]
    DEVICE_CLASS_FIELD_NUMBER: _ClassVar[int]
    DOCS_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    PACKAGES_FIELD_NUMBER: _ClassVar[int]
    PINS_FIELD_NUMBER: _ClassVar[int]
    RELATIONS_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    mpn: str
    manufacturer: str
    device_class: str
    docs: _containers.RepeatedCompositeFieldContainer[SourceDoc]
    parameters: _containers.RepeatedCompositeFieldContainer[Parameter]
    packages: _containers.RepeatedCompositeFieldContainer[Package]
    pins: _containers.RepeatedCompositeFieldContainer[Pin]
    relations: _containers.RepeatedCompositeFieldContainer[PinRelation]
    attributes: _containers.ScalarMap[str, str]
    def __init__(self, mpn: _Optional[str] = ..., manufacturer: _Optional[str] = ..., device_class: _Optional[str] = ..., docs: _Optional[_Iterable[_Union[SourceDoc, _Mapping]]] = ..., parameters: _Optional[_Iterable[_Union[Parameter, _Mapping]]] = ..., packages: _Optional[_Iterable[_Union[Package, _Mapping]]] = ..., pins: _Optional[_Iterable[_Union[Pin, _Mapping]]] = ..., relations: _Optional[_Iterable[_Union[PinRelation, _Mapping]]] = ..., attributes: _Optional[_Mapping[str, str]] = ...) -> None: ...

class Package(_message.Message):
    __slots__ = ("id", "name", "mpn_suffix", "attributes")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    MPN_SUFFIX_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    mpn_suffix: str
    attributes: _containers.ScalarMap[str, str]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., mpn_suffix: _Optional[str] = ..., attributes: _Optional[_Mapping[str, str]] = ...) -> None: ...

class PinNumber(_message.Message):
    __slots__ = ("package_ref", "number")
    PACKAGE_REF_FIELD_NUMBER: _ClassVar[int]
    NUMBER_FIELD_NUMBER: _ClassVar[int]
    package_ref: str
    number: str
    def __init__(self, package_ref: _Optional[str] = ..., number: _Optional[str] = ...) -> None: ...

class Pin(_message.Message):
    __slots__ = ("id", "name", "function", "description", "numbers", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    FUNCTION_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    NUMBERS_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    function: PinFunction
    description: str
    numbers: _containers.RepeatedCompositeFieldContainer[PinNumber]
    attributes: _containers.ScalarMap[str, str]
    prov: ParamProvenance
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., function: _Optional[_Union[PinFunction, str]] = ..., description: _Optional[str] = ..., numbers: _Optional[_Iterable[_Union[PinNumber, _Mapping]]] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[ParamProvenance, _Mapping]] = ...) -> None: ...

class PinRelation(_message.Message):
    __slots__ = ("subject_pin_ref", "reference_pin_ref", "kind", "difference", "unit", "modality", "conditions", "raw", "attributes", "prov")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    SUBJECT_PIN_REF_FIELD_NUMBER: _ClassVar[int]
    REFERENCE_PIN_REF_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    DIFFERENCE_FIELD_NUMBER: _ClassVar[int]
    UNIT_FIELD_NUMBER: _ClassVar[int]
    MODALITY_FIELD_NUMBER: _ClassVar[int]
    CONDITIONS_FIELD_NUMBER: _ClassVar[int]
    RAW_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    subject_pin_ref: str
    reference_pin_ref: str
    kind: PinRelationKind
    difference: RangeValue
    unit: str
    modality: Modality
    conditions: _containers.RepeatedCompositeFieldContainer[Condition]
    raw: str
    attributes: _containers.ScalarMap[str, str]
    prov: ParamProvenance
    def __init__(self, subject_pin_ref: _Optional[str] = ..., reference_pin_ref: _Optional[str] = ..., kind: _Optional[_Union[PinRelationKind, str]] = ..., difference: _Optional[_Union[RangeValue, _Mapping]] = ..., unit: _Optional[str] = ..., modality: _Optional[_Union[Modality, str]] = ..., conditions: _Optional[_Iterable[_Union[Condition, _Mapping]]] = ..., raw: _Optional[str] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[ParamProvenance, _Mapping]] = ...) -> None: ...

class SourceDoc(_message.Message):
    __slots__ = ("id", "title", "vendor", "locator", "content_hash")
    ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    VENDOR_FIELD_NUMBER: _ClassVar[int]
    LOCATOR_FIELD_NUMBER: _ClassVar[int]
    CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    id: str
    title: str
    vendor: str
    locator: str
    content_hash: str
    def __init__(self, id: _Optional[str] = ..., title: _Optional[str] = ..., vendor: _Optional[str] = ..., locator: _Optional[str] = ..., content_hash: _Optional[str] = ...) -> None: ...

class RangeValue(_message.Message):
    __slots__ = ("min", "typ", "max")
    MIN_FIELD_NUMBER: _ClassVar[int]
    TYP_FIELD_NUMBER: _ClassVar[int]
    MAX_FIELD_NUMBER: _ClassVar[int]
    min: float
    typ: float
    max: float
    def __init__(self, min: _Optional[float] = ..., typ: _Optional[float] = ..., max: _Optional[float] = ...) -> None: ...

class Condition(_message.Message):
    __slots__ = ("symbol", "eq", "min", "max", "unit", "raw")
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    EQ_FIELD_NUMBER: _ClassVar[int]
    MIN_FIELD_NUMBER: _ClassVar[int]
    MAX_FIELD_NUMBER: _ClassVar[int]
    UNIT_FIELD_NUMBER: _ClassVar[int]
    RAW_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    eq: float
    min: float
    max: float
    unit: str
    raw: str
    def __init__(self, symbol: _Optional[str] = ..., eq: _Optional[float] = ..., min: _Optional[float] = ..., max: _Optional[float] = ..., unit: _Optional[str] = ..., raw: _Optional[str] = ...) -> None: ...

class Parameter(_message.Message):
    __slots__ = ("name", "symbol", "canonical_id", "limit_kind", "value", "unit", "conditions", "condition_coverage", "applies_to", "pin_refs", "attributes", "prov", "verification")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    CANONICAL_ID_FIELD_NUMBER: _ClassVar[int]
    LIMIT_KIND_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    UNIT_FIELD_NUMBER: _ClassVar[int]
    CONDITIONS_FIELD_NUMBER: _ClassVar[int]
    CONDITION_COVERAGE_FIELD_NUMBER: _ClassVar[int]
    APPLIES_TO_FIELD_NUMBER: _ClassVar[int]
    PIN_REFS_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    PROV_FIELD_NUMBER: _ClassVar[int]
    VERIFICATION_FIELD_NUMBER: _ClassVar[int]
    name: str
    symbol: str
    canonical_id: str
    limit_kind: LimitKind
    value: RangeValue
    unit: str
    conditions: _containers.RepeatedCompositeFieldContainer[Condition]
    condition_coverage: ConditionCoverage
    applies_to: str
    pin_refs: _containers.RepeatedScalarFieldContainer[str]
    attributes: _containers.ScalarMap[str, str]
    prov: ParamProvenance
    verification: Verification
    def __init__(self, name: _Optional[str] = ..., symbol: _Optional[str] = ..., canonical_id: _Optional[str] = ..., limit_kind: _Optional[_Union[LimitKind, str]] = ..., value: _Optional[_Union[RangeValue, _Mapping]] = ..., unit: _Optional[str] = ..., conditions: _Optional[_Iterable[_Union[Condition, _Mapping]]] = ..., condition_coverage: _Optional[_Union[ConditionCoverage, str]] = ..., applies_to: _Optional[str] = ..., pin_refs: _Optional[_Iterable[str]] = ..., attributes: _Optional[_Mapping[str, str]] = ..., prov: _Optional[_Union[ParamProvenance, _Mapping]] = ..., verification: _Optional[_Union[Verification, _Mapping]] = ...) -> None: ...

class Verification(_message.Message):
    __slots__ = ("by", "doc_content_hash", "at", "note", "doc_revision")
    BY_FIELD_NUMBER: _ClassVar[int]
    DOC_CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    AT_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    DOC_REVISION_FIELD_NUMBER: _ClassVar[int]
    by: str
    doc_content_hash: str
    at: str
    note: str
    doc_revision: str
    def __init__(self, by: _Optional[str] = ..., doc_content_hash: _Optional[str] = ..., at: _Optional[str] = ..., note: _Optional[str] = ..., doc_revision: _Optional[str] = ...) -> None: ...

class ParamProvenance(_message.Message):
    __slots__ = ("doc_ref", "page", "table_or_figure", "method", "confidence")
    DOC_REF_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    TABLE_OR_FIGURE_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    doc_ref: str
    page: int
    table_or_figure: str
    method: str
    confidence: float
    def __init__(self, doc_ref: _Optional[str] = ..., page: _Optional[int] = ..., table_or_figure: _Optional[str] = ..., method: _Optional[str] = ..., confidence: _Optional[float] = ...) -> None: ...
