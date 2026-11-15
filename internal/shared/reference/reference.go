// Package reference 定义跨模块内联的引用实体快照。
//
// 读接口（列表 / 详情）需要把 `*_id` 之外的编码与名称一并返回，避免客户端再按 ID
// 二次查询后自行拼接。各模块的 `Reference` 都是本类型的别名，因此形状与 JSON
// 字段保持一致；写入与校验仍然走各自的防腐层与校验接口，不经过本包。
package reference

// Reference 是一个被引用实体的最小可读快照。
//
// 字段语义：
//   - Deleted 由引用表的 `deleted_at` 推导，用于告诉客户端「这个名字属于一条已软删除的记录」；
//   - Status 只在查询显式选出引用表状态时填充；空字符串表示本次查询未取该字段，
//     客户端不得据此判断「已停用」。
type Reference struct {
	ID      int64  `json:"id"`
	Code    string `json:"code"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
	Status  string `json:"status,omitempty"`
}

// FromJoin 由 LEFT JOIN 出的可空列构造引用快照。
//
// code 为 NULL 表示关联行不存在（外键约束下正常不会发生），此时返回 nil，
// 调用方靠 `omitempty` 省掉该字段而不是伪造一个空引用。
func FromJoin(id int64, code, name, status *string, deleted bool) *Reference {
	if code == nil {
		return nil
	}

	ref := &Reference{ID: id, Code: *code, Deleted: deleted}
	if name != nil {
		ref.Name = *name
	}
	if status != nil {
		ref.Status = *status
	}
	return ref
}
