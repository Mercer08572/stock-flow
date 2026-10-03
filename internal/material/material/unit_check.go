package material

/**
 * 计量单位可公度规则。
 *
 * 放在物料模块的理由：换算规则本身属于物料（「物料级单位换算」在 `internal/material/AGENTS.md`
 * 的范围里），而「两个单位能不能互相换算」是这条规则的判定，两个调用方都在本模块边界内：
 * 换算模块创建换算规则时校验，物料服务校验 SKU 单位时校验。
 *
 * 命名避免与 `internal/material/unit` 包的 `UnitType` 冲突，因此用同名值 + `Name` 后缀。
 */

/** 与 `units.unit_type` 的取值一一对应（见迁移的 chk_units_type） */
const (
	UnitTypeNameCount   = "count"
	UnitTypeNameWeight  = "weight"
	UnitTypeNameLength  = "length"
	UnitTypeNameArea    = "area"
	UnitTypeNameVolume  = "volume"
	UnitTypeNamePackage = "package"
	UnitTypeNameTime    = "time"
	UnitTypeNameOther   = "other"
)

/**
 * 两个计量单位是否可公度。
 *
 * 规则：`unit_type` 相同即可公度；额外允许「包装 ↔ 计数」
 * （`1 box = 12 pcs` 这类规则的单位类型天然不同，否则永远无法录入）。
 * weight↔count 这类真正会算错账的组合仍然被拒绝。
 */
func CommensurableUnitTypes(from string, to string) bool {
	if from == to {
		return true
	}

	return isPackageCountPair(from, to) || isPackageCountPair(to, from)
}

func isPackageCountPair(left string, right string) bool {
	return left == UnitTypeNamePackage && right == UnitTypeNameCount
}

/** 换算校验的结果分类；调用方据此映射自己的业务错误码 */
type UnitConversionCheckStatus string

const (
	/** 两个单位都存在且可公度 */
	UnitConversionOK UnitConversionCheckStatus = "ok"
	/** 起始单位不存在（或已软删除） */
	UnitConversionFromUnitMissing UnitConversionCheckStatus = "from_unit_missing"
	/** 目标单位不存在（或已软删除） */
	UnitConversionToUnitMissing UnitConversionCheckStatus = "to_unit_missing"
	/** 两个单位存在但单位类型不可公度 */
	UnitConversionUnitTypeMismatch UnitConversionCheckStatus = "unit_type_mismatch"
	/** 两个单位可公度，但没有一端是物料的基础单位 */
	UnitConversionBaseUnitRequired UnitConversionCheckStatus = "base_unit_required"
)

type UnitConversionCheck struct {
	Status     UnitConversionCheckStatus
	FromUnitID int64
	ToUnitID   int64
	FromType   string
	ToType     string
}

/** 校验通过，且方向被规范化为「基础单位 → 另一单位」 */
func (c UnitConversionCheck) IsOK() bool {
	return c.Status == UnitConversionOK
}

/** 是否触及物料的基础单位（调用方据此规范换算方向） */
func (c UnitConversionCheck) TouchesBaseUnit(baseUnitID int64) bool {
	return c.FromUnitID == baseUnitID || c.ToUnitID == baseUnitID
}

/** 返回规范化后的方向：始终让基础单位作为 `from` */
func (c UnitConversionCheck) NormalizedDirection(baseUnitID int64) (int64, int64) {
	if c.FromUnitID == baseUnitID {
		return c.FromUnitID, c.ToUnitID
	}
	return c.ToUnitID, c.FromUnitID
}
