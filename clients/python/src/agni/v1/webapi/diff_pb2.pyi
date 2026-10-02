from agni.v1.ir import ir_pb2 as _ir_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DiffDesignsRequest(_message.Message):
    __slots__ = ("a_uri", "b_uri", "near_renames")
    A_URI_FIELD_NUMBER: _ClassVar[int]
    B_URI_FIELD_NUMBER: _ClassVar[int]
    NEAR_RENAMES_FIELD_NUMBER: _ClassVar[int]
    a_uri: str
    b_uri: str
    near_renames: NearRenameOptions
    def __init__(self, a_uri: _Optional[str] = ..., b_uri: _Optional[str] = ..., near_renames: _Optional[_Union[NearRenameOptions, _Mapping]] = ...) -> None: ...

class NearRenameOptions(_message.Message):
    __slots__ = ("min_old_coverage", "min_old_coverage_significant", "min_new_coverage", "min_new_coverage_significant", "max_added_significant_floor", "min_significant_endpoints", "insignificant_classes")
    MIN_OLD_COVERAGE_FIELD_NUMBER: _ClassVar[int]
    MIN_OLD_COVERAGE_SIGNIFICANT_FIELD_NUMBER: _ClassVar[int]
    MIN_NEW_COVERAGE_FIELD_NUMBER: _ClassVar[int]
    MIN_NEW_COVERAGE_SIGNIFICANT_FIELD_NUMBER: _ClassVar[int]
    MAX_ADDED_SIGNIFICANT_FLOOR_FIELD_NUMBER: _ClassVar[int]
    MIN_SIGNIFICANT_ENDPOINTS_FIELD_NUMBER: _ClassVar[int]
    INSIGNIFICANT_CLASSES_FIELD_NUMBER: _ClassVar[int]
    min_old_coverage: float
    min_old_coverage_significant: float
    min_new_coverage: float
    min_new_coverage_significant: float
    max_added_significant_floor: int
    min_significant_endpoints: int
    insignificant_classes: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, min_old_coverage: _Optional[float] = ..., min_old_coverage_significant: _Optional[float] = ..., min_new_coverage: _Optional[float] = ..., min_new_coverage_significant: _Optional[float] = ..., max_added_significant_floor: _Optional[int] = ..., min_significant_endpoints: _Optional[int] = ..., insignificant_classes: _Optional[_Iterable[str]] = ...) -> None: ...

class DiffReport(_message.Message):
    __slots__ = ("components_added", "components_removed", "components_changed", "nets")
    class ComponentChange(_message.Message):
        __slots__ = ("ref_des", "field", "old", "new")
        REF_DES_FIELD_NUMBER: _ClassVar[int]
        FIELD_FIELD_NUMBER: _ClassVar[int]
        OLD_FIELD_NUMBER: _ClassVar[int]
        NEW_FIELD_NUMBER: _ClassVar[int]
        ref_des: str
        field: str
        old: str
        new: str
        def __init__(self, ref_des: _Optional[str] = ..., field: _Optional[str] = ..., old: _Optional[str] = ..., new: _Optional[str] = ...) -> None: ...
    class NetChange(_message.Message):
        __slots__ = ("kind", "name", "old_name", "added", "removed", "old_prov", "new_prov", "approx")
        KIND_FIELD_NUMBER: _ClassVar[int]
        NAME_FIELD_NUMBER: _ClassVar[int]
        OLD_NAME_FIELD_NUMBER: _ClassVar[int]
        ADDED_FIELD_NUMBER: _ClassVar[int]
        REMOVED_FIELD_NUMBER: _ClassVar[int]
        OLD_PROV_FIELD_NUMBER: _ClassVar[int]
        NEW_PROV_FIELD_NUMBER: _ClassVar[int]
        APPROX_FIELD_NUMBER: _ClassVar[int]
        kind: str
        name: str
        old_name: str
        added: _containers.RepeatedScalarFieldContainer[str]
        removed: _containers.RepeatedScalarFieldContainer[str]
        old_prov: _ir_pb2.Provenance
        new_prov: _ir_pb2.Provenance
        approx: DiffReport.RenameEvidence
        def __init__(self, kind: _Optional[str] = ..., name: _Optional[str] = ..., old_name: _Optional[str] = ..., added: _Optional[_Iterable[str]] = ..., removed: _Optional[_Iterable[str]] = ..., old_prov: _Optional[_Union[_ir_pb2.Provenance, _Mapping]] = ..., new_prov: _Optional[_Union[_ir_pb2.Provenance, _Mapping]] = ..., approx: _Optional[_Union[DiffReport.RenameEvidence, _Mapping]] = ...) -> None: ...
    class RenameEvidence(_message.Message):
        __slots__ = ("old_coverage", "old_coverage_significant", "new_coverage_significant", "overlap", "overlap_significant", "old_endpoints", "new_endpoints", "old_significant", "new_significant")
        OLD_COVERAGE_FIELD_NUMBER: _ClassVar[int]
        OLD_COVERAGE_SIGNIFICANT_FIELD_NUMBER: _ClassVar[int]
        NEW_COVERAGE_SIGNIFICANT_FIELD_NUMBER: _ClassVar[int]
        OVERLAP_FIELD_NUMBER: _ClassVar[int]
        OVERLAP_SIGNIFICANT_FIELD_NUMBER: _ClassVar[int]
        OLD_ENDPOINTS_FIELD_NUMBER: _ClassVar[int]
        NEW_ENDPOINTS_FIELD_NUMBER: _ClassVar[int]
        OLD_SIGNIFICANT_FIELD_NUMBER: _ClassVar[int]
        NEW_SIGNIFICANT_FIELD_NUMBER: _ClassVar[int]
        old_coverage: float
        old_coverage_significant: float
        new_coverage_significant: float
        overlap: int
        overlap_significant: int
        old_endpoints: int
        new_endpoints: int
        old_significant: int
        new_significant: int
        def __init__(self, old_coverage: _Optional[float] = ..., old_coverage_significant: _Optional[float] = ..., new_coverage_significant: _Optional[float] = ..., overlap: _Optional[int] = ..., overlap_significant: _Optional[int] = ..., old_endpoints: _Optional[int] = ..., new_endpoints: _Optional[int] = ..., old_significant: _Optional[int] = ..., new_significant: _Optional[int] = ...) -> None: ...
    COMPONENTS_ADDED_FIELD_NUMBER: _ClassVar[int]
    COMPONENTS_REMOVED_FIELD_NUMBER: _ClassVar[int]
    COMPONENTS_CHANGED_FIELD_NUMBER: _ClassVar[int]
    NETS_FIELD_NUMBER: _ClassVar[int]
    components_added: _containers.RepeatedScalarFieldContainer[str]
    components_removed: _containers.RepeatedScalarFieldContainer[str]
    components_changed: _containers.RepeatedCompositeFieldContainer[DiffReport.ComponentChange]
    nets: _containers.RepeatedCompositeFieldContainer[DiffReport.NetChange]
    def __init__(self, components_added: _Optional[_Iterable[str]] = ..., components_removed: _Optional[_Iterable[str]] = ..., components_changed: _Optional[_Iterable[_Union[DiffReport.ComponentChange, _Mapping]]] = ..., nets: _Optional[_Iterable[_Union[DiffReport.NetChange, _Mapping]]] = ...) -> None: ...

class DiffDesignsResponse(_message.Message):
    __slots__ = ("report", "component_status", "net_status", "component_sheets_a", "component_sheets_b", "net_sheets_a", "net_sheets_b", "shared_placements_a", "shared_placements_b")
    class ComponentStatusEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    class NetStatusEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    class SheetIds(_message.Message):
        __slots__ = ("ids",)
        IDS_FIELD_NUMBER: _ClassVar[int]
        ids: _containers.RepeatedScalarFieldContainer[str]
        def __init__(self, ids: _Optional[_Iterable[str]] = ...) -> None: ...
    class ComponentSheetsAEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: DiffDesignsResponse.SheetIds
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[DiffDesignsResponse.SheetIds, _Mapping]] = ...) -> None: ...
    class ComponentSheetsBEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: DiffDesignsResponse.SheetIds
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[DiffDesignsResponse.SheetIds, _Mapping]] = ...) -> None: ...
    class NetSheetsAEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: DiffDesignsResponse.SheetIds
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[DiffDesignsResponse.SheetIds, _Mapping]] = ...) -> None: ...
    class NetSheetsBEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: DiffDesignsResponse.SheetIds
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[DiffDesignsResponse.SheetIds, _Mapping]] = ...) -> None: ...
    class Placement(_message.Message):
        __slots__ = ("sheet", "x", "y")
        SHEET_FIELD_NUMBER: _ClassVar[int]
        X_FIELD_NUMBER: _ClassVar[int]
        Y_FIELD_NUMBER: _ClassVar[int]
        sheet: str
        x: float
        y: float
        def __init__(self, sheet: _Optional[str] = ..., x: _Optional[float] = ..., y: _Optional[float] = ...) -> None: ...
    class SharedPlacementsAEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: DiffDesignsResponse.Placement
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[DiffDesignsResponse.Placement, _Mapping]] = ...) -> None: ...
    class SharedPlacementsBEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: DiffDesignsResponse.Placement
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[DiffDesignsResponse.Placement, _Mapping]] = ...) -> None: ...
    REPORT_FIELD_NUMBER: _ClassVar[int]
    COMPONENT_STATUS_FIELD_NUMBER: _ClassVar[int]
    NET_STATUS_FIELD_NUMBER: _ClassVar[int]
    COMPONENT_SHEETS_A_FIELD_NUMBER: _ClassVar[int]
    COMPONENT_SHEETS_B_FIELD_NUMBER: _ClassVar[int]
    NET_SHEETS_A_FIELD_NUMBER: _ClassVar[int]
    NET_SHEETS_B_FIELD_NUMBER: _ClassVar[int]
    SHARED_PLACEMENTS_A_FIELD_NUMBER: _ClassVar[int]
    SHARED_PLACEMENTS_B_FIELD_NUMBER: _ClassVar[int]
    report: DiffReport
    component_status: _containers.ScalarMap[str, str]
    net_status: _containers.ScalarMap[str, str]
    component_sheets_a: _containers.MessageMap[str, DiffDesignsResponse.SheetIds]
    component_sheets_b: _containers.MessageMap[str, DiffDesignsResponse.SheetIds]
    net_sheets_a: _containers.MessageMap[str, DiffDesignsResponse.SheetIds]
    net_sheets_b: _containers.MessageMap[str, DiffDesignsResponse.SheetIds]
    shared_placements_a: _containers.MessageMap[str, DiffDesignsResponse.Placement]
    shared_placements_b: _containers.MessageMap[str, DiffDesignsResponse.Placement]
    def __init__(self, report: _Optional[_Union[DiffReport, _Mapping]] = ..., component_status: _Optional[_Mapping[str, str]] = ..., net_status: _Optional[_Mapping[str, str]] = ..., component_sheets_a: _Optional[_Mapping[str, DiffDesignsResponse.SheetIds]] = ..., component_sheets_b: _Optional[_Mapping[str, DiffDesignsResponse.SheetIds]] = ..., net_sheets_a: _Optional[_Mapping[str, DiffDesignsResponse.SheetIds]] = ..., net_sheets_b: _Optional[_Mapping[str, DiffDesignsResponse.SheetIds]] = ..., shared_placements_a: _Optional[_Mapping[str, DiffDesignsResponse.Placement]] = ..., shared_placements_b: _Optional[_Mapping[str, DiffDesignsResponse.Placement]] = ...) -> None: ...
