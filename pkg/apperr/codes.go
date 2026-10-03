package apperr

// 业务错误码表。
//
// 命名规范：<域>_<对象>_<原因>，全大写下划线分隔。
// 两条硬性规则：
//
//  1. 一码只对应一个 HTTP 状态。同一资源在不同语义下必须是不同码，
//     例如 MATERIAL_CATEGORY_NOT_FOUND（分类本身不存在，404）与
//     MATERIAL_CATEGORY_REFERENCE_INVALID（物料引用了不存在的分类，400）。
//  2. 码是对外契约：一旦发布就不要改字符串，改了等于破坏前端与外部调用方。
const (
	// 认证与授权
	CodeAuthInvalidCredentials     Code = "AUTH_INVALID_CREDENTIALS"
	CodeAuthRequired               Code = "AUTH_REQUIRED"
	CodeAuthSessionExpired         Code = "AUTH_SESSION_EXPIRED"
	CodeAuthAmbiguousMethod        Code = "AUTH_AMBIGUOUS_METHOD"
	CodeAuthPasswordChangeRequired Code = "AUTH_PASSWORD_CHANGE_REQUIRED"
	CodeAuthRateLimited            Code = "AUTH_RATE_LIMITED"
	CodeAuthAPIAppNotFound         Code = "AUTH_API_APP_NOT_FOUND"
	CodeAuthAPISecretNotFound      Code = "AUTH_API_SECRET_NOT_FOUND"
	CodeAuthAPIAppDuplicate        Code = "AUTH_API_APP_DUPLICATE"
	CodeAuthAPISecretDuplicate     Code = "AUTH_API_SECRET_DUPLICATE"

	// 仓库
	CodeWarehouseNotFound              Code = "WAREHOUSE_NOT_FOUND"
	CodeWarehouseCodeDuplicate         Code = "WAREHOUSE_CODE_DUPLICATE"
	CodeWarehouseReferencedByInventory Code = "WAREHOUSE_REFERENCED_BY_INVENTORY"

	// SKU
	CodeSKUNotFound                Code = "SKU_NOT_FOUND"
	CodeSKUCodeDuplicate           Code = "SKU_CODE_DUPLICATE"
	CodeSKUActiveExistsForMaterial Code = "SKU_ACTIVE_EXISTS_FOR_MATERIAL"
	CodeSKUReferencedByInventory   Code = "SKU_REFERENCED_BY_INVENTORY"
	CodeSKUMaterialNotFound        Code = "SKU_MATERIAL_NOT_FOUND"
	CodeSKUUnitNotFound            Code = "SKU_UNIT_NOT_FOUND"
	CodeSKUUnitNotAllowed          Code = "SKU_UNIT_NOT_ALLOWED"

	// 计量单位
	CodeMaterialUnitNotFound      Code = "MATERIAL_UNIT_NOT_FOUND"
	CodeMaterialUnitCodeDuplicate Code = "MATERIAL_UNIT_CODE_DUPLICATE"

	// 物料分类
	CodeMaterialCategoryNotFound       Code = "MATERIAL_CATEGORY_NOT_FOUND"
	CodeMaterialCategoryCodeDuplicate  Code = "MATERIAL_CATEGORY_CODE_DUPLICATE"
	CodeMaterialCategoryParentNotFound Code = "MATERIAL_CATEGORY_PARENT_NOT_FOUND"

	// 物料
	CodeMaterialNotFound                 Code = "MATERIAL_NOT_FOUND"
	CodeMaterialCodeDuplicate            Code = "MATERIAL_CODE_DUPLICATE"
	CodeMaterialCategoryReferenceInvalid Code = "MATERIAL_CATEGORY_REFERENCE_INVALID"
	CodeMaterialBaseUnitReferenceInvalid Code = "MATERIAL_BASE_UNIT_REFERENCE_INVALID"

	// 物料单位换算
	CodeMaterialConversionNotFound         Code = "MATERIAL_CONVERSION_NOT_FOUND"
	CodeMaterialConversionDuplicate        Code = "MATERIAL_CONVERSION_DUPLICATE"
	CodeMaterialConversionReverseDuplicate Code = "MATERIAL_CONVERSION_REVERSE_DUPLICATE"
	CodeMaterialConversionMaterialInvalid  Code = "MATERIAL_CONVERSION_MATERIAL_INVALID"
	CodeMaterialConversionFromUnitInvalid  Code = "MATERIAL_CONVERSION_FROM_UNIT_INVALID"
	CodeMaterialConversionToUnitInvalid    Code = "MATERIAL_CONVERSION_TO_UNIT_INVALID"
	CodeMaterialConversionUnitTypeMismatch Code = "MATERIAL_CONVERSION_UNIT_TYPE_MISMATCH"
	CodeMaterialConversionBaseUnitRequired Code = "MATERIAL_CONVERSION_BASE_UNIT_REQUIRED"

	// 库存
	CodeInventoryStockNotFound     Code = "INVENTORY_STOCK_NOT_FOUND"
	CodeInventoryWarehouseNotFound Code = "INVENTORY_WAREHOUSE_NOT_FOUND"
	CodeInventorySKUNotFound       Code = "INVENTORY_SKU_NOT_FOUND"
	CodeInventoryBatchNotFound     Code = "INVENTORY_BATCH_NOT_FOUND"
)
