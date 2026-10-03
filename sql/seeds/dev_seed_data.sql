-- =============================================================================
-- Stock-Flow 开发环境测试数据（dev seed）
-- =============================================================================
-- 用途：为本地/开发库（stock_flow_dev）一次性灌入可联调、可验证的业务数据，
--       覆盖主数据（物料域）与库存只读视图（批次 / 库存层 / 库存余额）。
--
-- 范围（11 张表，仅主数据 + 库存余额）：
--   units, material_categories, materials,
--   material_attribute_definitions, material_attribute_values,
--   material_unit_conversions, skus, warehouses,
--   inventory_batches, inventory_stock_layers, inventory_stocks
--
-- 明确不涉及：
--   admin_users（保留现有 admin 行）、api_apps / api_secrets、
--   inventory_reservations / inventory_reservation_items / inventory_movements /
--   inventory_idempotency_keys（按约定留空，待 P3 的库存写操作实现后再补）
--
-- 安全性：
--   * 单事务（BEGIN ... COMMIT）：任一步失败则整体回滚，不会留下半截数据。
--   * 只做 INSERT 与 setval，无 UPDATE / DELETE / TRUNCATE。
--   * 执行前先守卫检查：11 张目标表必须全为空，否则立即 RAISE EXCEPTION 中止。
--   * 结尾做不变量校验：库存余额必须等于库存层求和、FIFO 顺序必须符合预期，
--     不成立则抛异常并回滚。
--   * 结束时复位 11 个 BIGSERIAL 序列，避免应用后续插入撞主键。
--
-- 执行方式（psql）：
--   psql "$(cd stock-flow && go run ./cmd/config -key database_url)" \
--        -v ON_ERROR_STOP=1 -f stock-flow/sql/seeds/dev_seed_data.sql
--
--   # 容器内
--   docker exec -i <pg-container> psql -U postgres -d stock_flow_dev \
--        -v ON_ERROR_STOP=1 -f - < stock-flow/sql/seeds/dev_seed_data.sql
--
--   本文件含 psql 元命令（\set / \echo），其他客户端请删除这些行后再执行。
--   需要重灌时，取消文件末尾「清理块」的注释。
-- =============================================================================

\set ON_ERROR_STOP on

BEGIN;

\echo '>>> [1/6] 守卫检查：11 张目标表必须全为空'

DO $guard$
DECLARE
    target_tables CONSTANT text[] := ARRAY[
        'units',
        'material_categories',
        'materials',
        'material_attribute_definitions',
        'material_attribute_values',
        'material_unit_conversions',
        'skus',
        'warehouses',
        'inventory_batches',
        'inventory_stock_layers',
        'inventory_stocks'
    ];
    tbl        text;
    cnt        bigint;
    offenders  text := '';
BEGIN
    FOREACH tbl IN ARRAY target_tables LOOP
        EXECUTE format('SELECT count(*) FROM %I', tbl) INTO cnt;
        IF cnt > 0 THEN
            offenders := offenders || format('%s=%s ', tbl, cnt);
        END IF;
    END LOOP;

    IF offenders <> '' THEN
        RAISE EXCEPTION 'dev seed aborted: target tables are not empty -> %', offenders
            USING HINT = '目标表必须为空。请先取消本文件末尾「清理块」的注释并执行，再重跑本脚本。';
    END IF;

    RAISE NOTICE 'guard passed: all % target tables are empty', array_length(target_tables, 1);
END
$guard$;

-- =============================================================================
-- [2/6] 主数据：计量单位 / 物料分类
-- =============================================================================

\echo '>>> [2/6] 写入计量单位、物料分类'

-- 覆盖 unit_type 的 7 种取值（count/package/weight/length/area/volume/time/other），
-- LB 为 inactive，用于验证状态筛选。
INSERT INTO units (id, code, name, symbol, unit_type, precision, status, created_at, updated_at) VALUES
    ( 1, 'PCS', '个',   'pcs', 'count',   0, 'active',   '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    ( 2, 'BOX', '箱',   'box', 'package', 0, 'active',   '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    ( 3, 'KG',  '千克', 'kg',  'weight',  3, 'active',   '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    ( 4, 'G',   '克',   'g',   'weight',  0, 'active',   '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    ( 5, 'M',   '米',   'm',   'length',  2, 'active',   '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    ( 6, 'CM',  '厘米', 'cm',  'length',  1, 'active',   '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    ( 7, 'M2',  '平方米', 'm2', 'area',   2, 'active',   '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    ( 8, 'L',   '升',   'L',   'volume',  3, 'active',   '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    ( 9, 'MIN', '分钟', 'min', 'time',    0, 'active',   '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    (10, 'SET', '套',   'set', 'other',   0, 'active',   '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    (11, 'LB',  '磅',   'lb',  'weight',  3, 'inactive', '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08');

-- 三级分类：RAW(1) -> RAW-METAL(4) / RAW-PLASTIC(5)；FIN(3) -> FIN-ELEC(6)；
-- AUX(7) 为 inactive，用于验证分类停用与筛选。
INSERT INTO material_categories (id, code, name, parent_id, status, remark, created_at, updated_at) VALUES
    (1, 'RAW',         '原材料',     NULL, 'active',   NULL,                                       '2025-01-06 09:10:00+08', '2025-01-06 09:10:00+08'),
    (2, 'SEMI',        '半成品',     NULL, 'active',   NULL,                                       '2025-01-06 09:10:00+08', '2025-01-06 09:10:00+08'),
    (3, 'FIN',         '成品',       NULL, 'active',   NULL,                                       '2025-01-06 09:10:00+08', '2025-01-06 09:10:00+08'),
    (4, 'RAW-METAL',   '金属原材料', 1,    'active',   '钢板、铝材、铜材等',                        '2025-01-06 09:10:00+08', '2025-01-06 09:10:00+08'),
    (5, 'RAW-PLASTIC', '塑料原料',   1,    'active',   '塑料粒子、树脂等',                          '2025-01-06 09:10:00+08', '2025-01-06 09:10:00+08'),
    (6, 'FIN-ELEC',    '电子产品',   3,    'active',   '整机电子类成品',                            '2025-01-06 09:10:00+08', '2025-01-06 09:10:00+08'),
    (7, 'AUX',         '辅料',       NULL, 'inactive', '已停用分类，保留历史数据，用于验证状态筛选', '2025-01-06 09:10:00+08', '2025-01-06 09:10:00+08');

-- =============================================================================
-- [3/6] 主数据：物料 / 属性定义 / 属性值 / 单位换算
-- =============================================================================

\echo '>>> [3/6] 写入物料、属性定义、属性值、单位换算'

-- 12 个物料：11 active + 1 inactive(AUX-TAPE-01)；
-- MAT-AL-5052 故意不建 SKU，用于验证「无 SKU 物料」场景。
INSERT INTO materials (id, code, name, category_id, base_unit_id, status, remark, created_at, updated_at) VALUES
    ( 1, 'MAT-AL-6061',   '铝板 6061',         4,  3, 'active',   '常用铝材，按千克计量',       '2025-01-07 09:00:00+08', '2025-01-07 09:00:00+08'),
    ( 2, 'MAT-STEEL-Q235', '碳钢板 Q235',      4,  3, 'active',   '非批次管理示例',             '2025-01-07 09:00:00+08', '2025-01-07 09:00:00+08'),
    ( 3, 'MAT-ABS-750',   'ABS 塑料粒子 750',  5,  3, 'active',   '批次管理示例，含过期批次',   '2025-01-07 09:00:00+08', '2025-01-07 09:00:00+08'),
    ( 4, 'MAT-PP-K8003',  'PP 塑料粒子 K8003', 5,  3, 'active',   NULL,                         '2025-01-07 09:00:00+08', '2025-01-07 09:00:00+08'),
    ( 5, 'MAT-GLUE-E05',  '环氧树脂胶 E-05',   1,  8, 'active',   '按升计量',                   '2025-01-07 09:00:00+08', '2025-01-07 09:00:00+08'),
    ( 6, 'SEMI-BOARD-A',  '控制板半成品 A',    2,  1, 'active',   NULL,                         '2025-01-07 09:00:00+08', '2025-01-07 09:00:00+08'),
    ( 7, 'SEMI-CASE-B',   '外壳半成品 B',      2,  1, 'active',   NULL,                         '2025-01-07 09:00:00+08', '2025-01-07 09:00:00+08'),
    ( 8, 'FIN-FAN-120',   '散热风扇 120mm',    6,  1, 'active',   '跨两个仓库 + 虚拟仓在途',    '2025-01-07 09:00:00+08', '2025-01-07 09:00:00+08'),
    ( 9, 'FIN-CTRL-X1',   '控制器 X1',         6, 10, 'active',   '按套计量，用于满预留边界',   '2025-01-07 09:00:00+08', '2025-01-07 09:00:00+08'),
    (10, 'FIN-CABLE-C1',  '连接线 C1',         3,  5, 'active',   '按米计量，验证小数精度',     '2025-01-07 09:00:00+08', '2025-01-07 09:00:00+08'),
    (11, 'AUX-TAPE-01',   '封箱胶带',          7,  1, 'inactive', '已停用物料，无 SKU',         '2025-01-07 09:00:00+08', '2025-01-07 09:00:00+08'),
    (12, 'MAT-AL-5052',   '铝卷 5052',         4,  3, 'active',   'active 但未建 SKU',          '2025-01-07 09:00:00+08', '2025-01-07 09:00:00+08');

-- 9 个属性定义：覆盖 text / number / boolean / option / date 五种 data_type；
-- 第 9 条为 inactive；第 1 条 required=true（其分类下 3 个物料均已填值）。
INSERT INTO material_attribute_definitions
    (id, category_id, code, name, data_type, required, status, created_at, updated_at) VALUES
    (1, 4, 'grade',           '牌号',         'text',    TRUE,  'active',   '2025-01-07 09:20:00+08', '2025-01-07 09:20:00+08'),
    (2, 4, 'thickness_mm',    '厚度(mm)',     'number',  FALSE, 'active',   '2025-01-07 09:20:00+08', '2025-01-07 09:20:00+08'),
    (3, 4, 'hazardous',       '是否危化品',   'boolean', FALSE, 'active',   '2025-01-07 09:20:00+08', '2025-01-07 09:20:00+08'),
    (4, 5, 'resin_code',      '树脂牌号',     'text',    FALSE, 'active',   '2025-01-07 09:20:00+08', '2025-01-07 09:20:00+08'),
    (5, 5, 'melt_index',      '熔融指数',     'number',  FALSE, 'active',   '2025-01-07 09:20:00+08', '2025-01-07 09:20:00+08'),
    (6, 6, 'color',           '颜色',         'option',  FALSE, 'active',   '2025-01-07 09:20:00+08', '2025-01-07 09:20:00+08'),
    (7, 6, 'warranty_months', '保修月数',     'number',  FALSE, 'active',   '2025-01-07 09:20:00+08', '2025-01-07 09:20:00+08'),
    (8, 6, 'release_date',    '上市日期',     'date',    FALSE, 'active',   '2025-01-07 09:20:00+08', '2025-01-07 09:20:00+08'),
    (9, 1, 'legacy_code',     '旧编码(停用)', 'text',    FALSE, 'inactive', '2025-01-07 09:20:00+08', '2025-01-07 09:20:00+08');

-- 16 条属性值；每条仅一个取值列非空（符合 chk_material_attribute_values_single_value）。
INSERT INTO material_attribute_values
    (id, material_id, definition_id, value_text, value_number, value_boolean, value_date, created_at, updated_at) VALUES
    ( 1,  1, 1, '6061-T6',  NULL,  NULL,  NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    ( 2,  1, 2, NULL,       3.0,   NULL,  NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    ( 3,  1, 3, NULL,       NULL,  FALSE, NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    ( 4,  2, 1, 'Q235B',    NULL,  NULL,  NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    ( 5,  2, 2, NULL,       5.0,   NULL,  NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    ( 6,  2, 3, NULL,       NULL,  FALSE, NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    ( 7, 12, 1, '5052-H32', NULL,  NULL,  NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    ( 8, 12, 2, NULL,       1.5,   NULL,  NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    ( 9,  3, 4, '750A',     NULL,  NULL,  NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    (10,  3, 5, NULL,       12.5,  NULL,  NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    (11,  4, 4, 'K8003',    NULL,  NULL,  NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    (12,  4, 5, NULL,       8.2,   NULL,  NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    (13,  8, 6, '黑色',     NULL,  NULL,  NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    (14,  8, 7, NULL,       12,    NULL,  NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    (15,  9, 6, '白色',     NULL,  NULL,  NULL, '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08'),
    (16,  9, 8, NULL,       NULL,  NULL,  '2025-03-15', '2025-01-08 09:00:00+08', '2025-01-08 09:00:00+08');

-- 5 条换算规则：双向（KG<->G、M<->CM）+ 跨体系（KG->LB，LB 为 inactive 单位）。
INSERT INTO material_unit_conversions
    (id, material_id, from_unit_id, to_unit_id, factor, created_at, updated_at) VALUES
    (1,  1, 3,  4, 1000.0000000000,   '2025-01-08 09:30:00+08', '2025-01-08 09:30:00+08'),
    (2,  1, 4,  3, 0.0010000000,      '2025-01-08 09:30:00+08', '2025-01-08 09:30:00+08'),
    (3,  1, 3, 11, 2.2046226218,      '2025-01-08 09:30:00+08', '2025-01-08 09:30:00+08'),
    (4, 10, 5,  6, 100.0000000000,    '2025-01-08 09:30:00+08', '2025-01-08 09:30:00+08'),
    (5, 10, 6,  5, 0.0100000000,      '2025-01-08 09:30:00+08', '2025-01-08 09:30:00+08');

-- =============================================================================
-- [4/6] 主数据：SKU / 仓库
-- =============================================================================

\echo '>>> [4/6] 写入 SKU、仓库'

-- 12 个 SKU：10 active（每条 SKU 的单位都等于其物料基础单位；物料 8、9 另有一条 inactive 遗留 SKU）
-- + 2 个 inactive 遗留 SKU（物料 8、9），用于验证同一物料存在多条非 active SKU。
INSERT INTO skus (id, material_id, code, name, unit_id, status, remark, created_at, updated_at) VALUES
    ( 1,  1, 'SKU-AL-6061',         '铝板 6061（千克）',   3,  'active',   NULL,                       '2025-01-09 09:00:00+08', '2025-01-09 09:00:00+08'),
    ( 2,  2, 'SKU-STEEL-Q235',      '碳钢板 Q235（千克）', 3,  'active',   '非批次管理',               '2025-01-09 09:00:00+08', '2025-01-09 09:00:00+08'),
    ( 3,  3, 'SKU-ABS-750',         'ABS 粒子 750（千克）', 3, 'active',   '批次管理',                 '2025-01-09 09:00:00+08', '2025-01-09 09:00:00+08'),
    ( 4,  4, 'SKU-PP-K8003',        'PP 粒子 K8003（千克）', 3, 'active',  '有 SKU 但无库存',          '2025-01-09 09:00:00+08', '2025-01-09 09:00:00+08'),
    ( 5,  5, 'SKU-GLUE-E05',        '环氧树脂胶 E-05（升）', 8, 'active',  NULL,                       '2025-01-09 09:00:00+08', '2025-01-09 09:00:00+08'),
    ( 6,  6, 'SKU-SEMI-BOARD-A',    '控制板半成品 A（个）', 1, 'active',  NULL,                       '2025-01-09 09:00:00+08', '2025-01-09 09:00:00+08'),
    ( 7,  7, 'SKU-SEMI-CASE-B',     '外壳半成品 B（个）',  1,  'active',   NULL,                       '2025-01-09 09:00:00+08', '2025-01-09 09:00:00+08'),
    ( 8,  8, 'SKU-FAN-120',         '散热风扇 120mm（个）', 1, 'active',  '跨仓 + 虚拟仓在途',        '2025-01-09 09:00:00+08', '2025-01-09 09:00:00+08'),
    ( 9,  9, 'SKU-CTRL-X1',         '控制器 X1（套）',     10, 'active',   '满预留边界数据',           '2025-01-09 09:00:00+08', '2025-01-09 09:00:00+08'),
    (10, 10, 'SKU-CABLE-C1',        '连接线 C1（米）',     5,  'active',   '小数数量与精度',           '2025-01-09 09:00:00+08', '2025-01-09 09:00:00+08'),
    (11,  8, 'SKU-FAN-120-LEGACY',  '散热风扇 120mm（旧）', 1, 'inactive', '已停用 SKU（同物料第 2 条）', '2025-01-09 09:00:00+08', '2025-01-09 09:00:00+08'),
    (12,  9, 'SKU-CTRL-X1-LEGACY',  '控制器 X1（旧）',     10, 'inactive', '已停用 SKU（同物料第 2 条）', '2025-01-09 09:00:00+08', '2025-01-09 09:00:00+08');

-- 6 个仓库：4 个 normal active + 1 个 virtual active（WH-TRANSIT）+ 1 个 inactive（WH-OLD）。
-- WH-GZ 无任何库存，用于空列表/空详情；WH-OLD 有库存且已停用，用于验证停用仓仍可查询。
INSERT INTO warehouses (id, code, name, type, status, location, contact_name, contact_phone, remark, created_at, updated_at) VALUES
    (1, 'WH-SH',      '上海中心仓',   'normal',  'active',   '上海市浦东新区川沙路 100 号',       '张伟', '13800000001', '主力仓，库存层最多',             '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    (2, 'WH-BJ',      '北京北方仓',   'normal',  'active',   '北京市大兴区物流园区 A 区',         '李强', '13800000002', NULL,                             '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    (3, 'WH-GZ',      '广州华南仓',   'normal',  'active',   '广州市白云区太和镇仓储路 8 号',     '陈敏', '13800000003', '暂无库存，用于空状态验证',       '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    (4, 'WH-TRANSIT', '在途虚拟仓',   'virtual', 'active',   NULL,                                NULL,   NULL,          '记录已发货未到货的在途库存',     '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    (5, 'WH-SZ',      '深圳电商仓',   'normal',  'active',   '深圳市宝安区福永街道物流园 3 栋',   '王芳', '13800000004', '暂无库存，用于空状态验证',       '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08'),
    (6, 'WH-OLD',     '苏州旧仓',     'normal',  'inactive', '苏州市吴中区老仓库路 1 号',         '赵磊', '13800000005', '已撤仓停用但仍有历史库存，禁止删除', '2025-01-06 09:00:00+08', '2025-01-06 09:00:00+08');

-- =============================================================================
-- [5/6] 库存：批次 / 库存层 / 库存余额
-- =============================================================================

\echo '>>> [5/6] 写入批次、库存层，并派生库存余额'

-- 7 个批次（仅批次管理的 SKU 1 / 3）：
--   B-20250510 / B-20250915  正常批次
--   B-EXPIRED                已过期批次（仍有实物库存，用于效期逻辑）
--   B-AL-PLAN                未收货（first_received_at 为 NULL）
--   B-OLD-RECALL             已停用批次（无库存层）
INSERT INTO inventory_batches
    (id, sku_id, batch_no, first_received_at, production_date, expiration_date, status, remark, created_at, updated_at) VALUES
    (1, 1, 'B-20250510',  '2025-05-10 09:00:00+08', '2025-05-01', '2027-04-30', 'active',   NULL,                       '2025-05-10 09:00:00+08', '2025-05-10 09:00:00+08'),
    (2, 1, 'B-20250915',  '2025-09-15 10:30:00+08', '2025-09-01', '2027-08-31', 'active',   NULL,                       '2025-09-15 10:30:00+08', '2025-09-15 10:30:00+08'),
    (3, 1, 'B-EXPIRED',   '2025-06-30 14:00:00+08', '2024-01-05', '2025-06-30', 'active',   '已过期，仍有实物库存',       '2025-06-30 14:00:00+08', '2025-06-30 14:00:00+08'),
    (4, 1, 'B-AL-PLAN',   NULL,                    NULL,         NULL,         'active',   '计划到货，尚未收货',         '2025-01-10 09:00:00+08', '2025-01-10 09:00:00+08'),
    (5, 3, 'B-ABS-2503',  '2025-03-05 09:00:00+08', '2025-02-20', '2026-02-19', 'active',   NULL,                       '2025-03-05 09:00:00+08', '2025-03-05 09:00:00+08'),
    (6, 3, 'B-ABS-2508',  '2025-08-12 11:00:00+08', '2025-08-01', '2026-07-31', 'active',   NULL,                       '2025-08-12 11:00:00+08', '2025-08-12 11:00:00+08'),
    (7, 1, 'B-OLD-RECALL', '2024-10-01 09:00:00+08', '2024-09-01', '2026-08-31', 'inactive', '已停用批次，库存已清空', '2024-10-01 09:00:00+08', '2024-10-01 09:00:00+08');

-- 16 个库存层，关键场景：
--   * 层 1/2/3：WH-SH 的铝板，received_at 与批号顺序不一致
--                （B-20250510 05-10 -> B-EXPIRED 06-30 -> B-20250915 09-15），
--                若按批号而非 received_at 排序的 FIFO 实现会立即暴露。
--   * 层 7/8：received_at 完全相同，用于验证 (received_at, id) 次序。
--   * 层 9：on_hand_qty = 0 的已耗尽层。
--   * 层 12：reserved_qty = on_hand_qty（可用量为 0 的边界）。
--   * 层 13：小数数量（1000.5 / 0.25）。
--   * 层 10/11/14：同一 SKU 分布在不同仓库（含虚拟仓与停用仓）。
--   * 层 4/5/6/15/16：批次层与非批次层混用。
INSERT INTO inventory_stock_layers
    (id, warehouse_id, sku_id, batch_id, received_at, on_hand_qty, reserved_qty, created_at, updated_at) VALUES
    ( 1, 1, 1,    1, '2025-05-10 09:00:00+08',  500.000000, 100.000000, '2025-05-10 09:00:00+08', '2025-05-10 09:00:00+08'),
    ( 2, 1, 1,    2, '2025-09-15 10:30:00+08',  300.000000,   0.000000, '2025-09-15 10:30:00+08', '2025-09-15 10:30:00+08'),
    ( 3, 1, 1,    3, '2025-06-30 14:00:00+08',  120.000000,   0.000000, '2025-06-30 14:00:00+08', '2025-06-30 14:00:00+08'),
    ( 4, 2, 1,    2, '2025-09-20 09:00:00+08',  200.000000,  50.000000, '2025-09-20 09:00:00+08', '2025-09-20 09:00:00+08'),
    ( 5, 1, 3,    5, '2025-03-05 09:00:00+08',  800.000000, 200.000000, '2025-03-05 09:00:00+08', '2025-03-05 09:00:00+08'),
    ( 6, 1, 3,    6, '2025-08-12 11:00:00+08',  600.000000,   0.000000, '2025-08-12 11:00:00+08', '2025-08-12 11:00:00+08'),
    ( 7, 1, 2, NULL, '2025-04-01 08:00:00+08', 1500.000000,   0.000000, '2025-04-01 08:00:00+08', '2025-04-01 08:00:00+08'),
    ( 8, 1, 2, NULL, '2025-04-01 08:00:00+08',  800.000000, 300.000000, '2025-04-01 08:00:00+08', '2025-04-01 08:00:00+08'),
    ( 9, 1, 2, NULL, '2025-07-15 16:00:00+08',    0.000000,   0.000000, '2025-07-15 16:00:00+08', '2025-07-15 16:00:00+08'),
    (10, 4, 8, NULL, '2025-10-01 09:00:00+08',   60.000000,   0.000000, '2025-10-01 09:00:00+08', '2025-10-01 09:00:00+08'),
    (11, 1, 8, NULL, '2025-10-12 14:30:00+08',   45.000000,  10.000000, '2025-10-12 14:30:00+08', '2025-10-12 14:30:00+08'),
    (12, 1, 9, NULL, '2025-06-01 10:00:00+08',   25.000000,  25.000000, '2025-06-01 10:00:00+08', '2025-06-01 10:00:00+08'),
    (13, 1, 10, NULL, '2025-02-10 09:30:00+08', 1000.500000,   0.250000, '2025-02-10 09:30:00+08', '2025-02-10 09:30:00+08'),
    (14, 6, 8, NULL, '2024-11-11 09:00:00+08',   40.000000,   0.000000, '2024-11-11 09:00:00+08', '2024-11-11 09:00:00+08'),
    (15, 2, 6, NULL, '2025-03-20 09:00:00+08',  300.000000, 120.000000, '2025-03-20 09:00:00+08', '2025-03-20 09:00:00+08'),
    (16, 2, 6, NULL, '2025-11-20 09:00:00+08',  150.000000,   0.000000, '2025-11-20 09:00:00+08', '2025-11-20 09:00:00+08');

-- 库存余额由库存层聚合派生，保证 on_hand_qty / reserved_qty 与层明细恒等，
-- 同时天然满足 reserved_qty <= on_hand_qty 与 >= 0。
INSERT INTO inventory_stocks
    (warehouse_id, sku_id, on_hand_qty, reserved_qty, created_at, updated_at)
SELECT warehouse_id,
       sku_id,
       SUM(on_hand_qty)  AS on_hand_qty,
       SUM(reserved_qty) AS reserved_qty,
       MIN(received_at)  AS created_at,
       MAX(received_at)  AS updated_at
FROM inventory_stock_layers
WHERE deleted_at IS NULL
GROUP BY warehouse_id, sku_id;

-- =============================================================================
-- [6/6] 复位序列 + 不变量校验
-- =============================================================================

\echo '>>> [6/6] 复位序列并校验不变量'

-- 显式写入 id 后必须复位序列，否则应用后续 INSERT 会撞主键。
DO $seqsync$
DECLARE
    tbl    text;
    seq    text;
    max_id bigint;
BEGIN
    FOREACH tbl IN ARRAY ARRAY[
        'units',
        'material_categories',
        'materials',
        'material_attribute_definitions',
        'material_attribute_values',
        'material_unit_conversions',
        'skus',
        'warehouses',
        'inventory_batches',
        'inventory_stock_layers',
        'inventory_stocks'
    ] LOOP
        seq := pg_get_serial_sequence(tbl, 'id');
        EXECUTE format('SELECT COALESCE(max(id), 0) FROM %I', tbl) INTO max_id;
        IF max_id > 0 THEN
            PERFORM setval(seq, max_id, TRUE);
        ELSE
            PERFORM setval(seq, 1, FALSE);
        END IF;
    END LOOP;

    RAISE NOTICE 'sequences realigned for 11 tables';
END
$seqsync$;

-- 校验 1：库存余额必须等于库存层求和（双向核对）。
DO $check_balance$
DECLARE
    mismatched bigint;
    orphan     bigint;
BEGIN
    SELECT count(*) INTO mismatched
    FROM inventory_stocks s
    LEFT JOIN (
        SELECT warehouse_id, sku_id,
               SUM(on_hand_qty)  AS layer_on_hand,
               SUM(reserved_qty) AS layer_reserved
        FROM inventory_stock_layers
        WHERE deleted_at IS NULL
        GROUP BY warehouse_id, sku_id
    ) l ON l.warehouse_id = s.warehouse_id AND l.sku_id = s.sku_id
    WHERE s.deleted_at IS NULL
      AND (
            s.on_hand_qty  <> COALESCE(l.layer_on_hand, 0)
         OR s.reserved_qty <> COALESCE(l.layer_reserved, 0)
      );

    IF mismatched > 0 THEN
        RAISE EXCEPTION 'seed invariant failed: % stock balance row(s) mismatch layer sum', mismatched;
    END IF;

    SELECT count(*) INTO orphan
    FROM (
        SELECT DISTINCT warehouse_id, sku_id
        FROM inventory_stock_layers
        WHERE deleted_at IS NULL
    ) l
    LEFT JOIN inventory_stocks s
           ON s.warehouse_id = l.warehouse_id
          AND s.sku_id = l.sku_id
          AND s.deleted_at IS NULL
    WHERE s.id IS NULL;

    IF orphan > 0 THEN
        RAISE EXCEPTION 'seed invariant failed: % (warehouse, sku) pair(s) have layers but no stock balance row', orphan;
    END IF;

    RAISE NOTICE 'invariant ok: stock balance == layer sum';
END
$check_balance$;

-- 校验 2：FIFO 顺序与预期一致（按 received_at, id 排序）。
-- 期望：B-20250510(05-10) -> B-EXPIRED(06-30) -> B-20250915(09-15)
DO $check_fifo$
DECLARE
    got text;
BEGIN
    SELECT string_agg(b.batch_no, ',' ORDER BY l.received_at, l.id) INTO got
    FROM inventory_stock_layers l
    JOIN inventory_batches b ON b.id = l.batch_id
    WHERE l.warehouse_id = 1
      AND l.sku_id = 1
      AND l.deleted_at IS NULL;

    IF COALESCE(got, '') <> 'B-20250510,B-EXPIRED,B-20250915' THEN
        RAISE EXCEPTION 'seed invariant failed: unexpected FIFO order for WH-SH / SKU-AL-6061 -> %', COALESCE(got, '<empty>');
    END IF;

    RAISE NOTICE 'invariant ok: FIFO order for WH-SH / SKU-AL-6061 is by received_at';
END
$check_fifo$;

COMMIT;

\echo ''
\echo '============================================================================='
\echo '种子数据写入成功。以下为核对输出（只读）。'
\echo '============================================================================='

\echo ''
\echo '--- 1. 各表行数（期望见注释）---'

SELECT table_name, row_count
FROM (
    SELECT 1  AS ord, 'units'                           AS table_name, count(*) AS row_count FROM units
    UNION ALL SELECT  2, 'material_categories',              count(*) FROM material_categories
    UNION ALL SELECT  3, 'materials',                        count(*) FROM materials
    UNION ALL SELECT  4, 'material_attribute_definitions',   count(*) FROM material_attribute_definitions
    UNION ALL SELECT  5, 'material_attribute_values',        count(*) FROM material_attribute_values
    UNION ALL SELECT  6, 'material_unit_conversions',        count(*) FROM material_unit_conversions
    UNION ALL SELECT  7, 'skus',                             count(*) FROM skus
    UNION ALL SELECT  8, 'warehouses',                       count(*) FROM warehouses
    UNION ALL SELECT  9, 'inventory_batches',                count(*) FROM inventory_batches
    UNION ALL SELECT 10, 'inventory_stock_layers',           count(*) FROM inventory_stock_layers
    UNION ALL SELECT 11, 'inventory_stocks',                 count(*) FROM inventory_stocks
    UNION ALL SELECT 12, 'inventory_reservations',           count(*) FROM inventory_reservations
    UNION ALL SELECT 13, 'inventory_movements',              count(*) FROM inventory_movements
    UNION ALL SELECT 14, 'inventory_idempotency_keys',       count(*) FROM inventory_idempotency_keys
) t
ORDER BY ord;
-- 期望：units=11  material_categories=7  materials=12  attr_defs=9  attr_values=16
--       conversions=5  skus=12  warehouses=6  batches=7  layers=16  stocks=10
--       reservations=0  movements=0  idempotency_keys=0（按约定留空）

\echo ''
\echo '--- 2. 库存余额（含可用量）---'

SELECT w.code                                             AS warehouse,
       s.code                                             AS sku,
       st.on_hand_qty,
       st.reserved_qty,
       (st.on_hand_qty - st.reserved_qty)                 AS available_qty,
       (SELECT count(*) FROM inventory_stock_layers l
         WHERE l.warehouse_id = st.warehouse_id
           AND l.sku_id = st.sku_id
           AND l.deleted_at IS NULL)                      AS layer_count
FROM inventory_stocks st
JOIN warehouses w ON w.id = st.warehouse_id
JOIN skus s       ON s.id = st.sku_id
ORDER BY w.code, s.code;
-- 期望 10 行；WH-SH / SKU-CTRL-X1 的 available_qty = 0（满预留边界）

\echo ''
\echo '--- 3. FIFO 顺序抽样：WH-SH / SKU-AL-6061 ---'

SELECT l.id,
       l.received_at,
       COALESCE(b.batch_no, '(无批次)') AS batch_no,
       l.on_hand_qty,
       l.reserved_qty,
       (l.on_hand_qty - l.reserved_qty) AS available_qty
FROM inventory_stock_layers l
LEFT JOIN inventory_batches b ON b.id = l.batch_id
WHERE l.warehouse_id = 1
  AND l.sku_id = 1
  AND l.deleted_at IS NULL
ORDER BY l.received_at, l.id;
-- 期望顺序：B-20250510 -> B-EXPIRED -> B-20250915

\echo ''
\echo '--- 4. 不变量核对（以下两个查询都必须返回 0）---'

SELECT count(*) AS balance_mismatch_count
FROM inventory_stocks s
LEFT JOIN (
    SELECT warehouse_id, sku_id,
           SUM(on_hand_qty)  AS layer_on_hand,
           SUM(reserved_qty) AS layer_reserved
    FROM inventory_stock_layers
    WHERE deleted_at IS NULL
    GROUP BY warehouse_id, sku_id
) l ON l.warehouse_id = s.warehouse_id AND l.sku_id = s.sku_id
WHERE s.deleted_at IS NULL
  AND (s.on_hand_qty <> COALESCE(l.layer_on_hand, 0)
    OR s.reserved_qty <> COALESCE(l.layer_reserved, 0));

SELECT count(*) AS invalid_qty_count
FROM inventory_stocks
WHERE on_hand_qty < 0
   OR reserved_qty < 0
   OR reserved_qty > on_hand_qty;

\echo ''
\echo '完成：开发测试数据已就绪。'
\echo '前端核对：make run（后端）+ pnpm dev（前端）后访问'
\echo '  /master-data/units        期望 11 行'
\echo '  /master-data/categories   期望 7 行'
\echo '  /master-data/materials    期望 12 行'
\echo '  /master-data/skus         期望 12 行'
\echo '  /master-data/warehouses   期望 6 行'
\echo '  /inventory/stocks         期望 10 行'
\echo ''

-- =============================================================================
-- 清理块（默认注释）：仅在需要重灌本脚本时取消注释后执行。
-- 顺序按外键依赖，从叶子表到主表；不会触碰 admin_users、api_* 与预留/流水表。
-- 若库里已有 inventory_reservations / inventory_movements 引用这些仓库或 SKU，
-- 本块会在这些表处失败并整体回滚——那是预期的保护，请先自行确认。
-- =============================================================================
/*
BEGIN;

DELETE FROM inventory_stocks;
DELETE FROM inventory_stock_layers;
DELETE FROM inventory_batches;
DELETE FROM skus;
DELETE FROM material_attribute_values;
DELETE FROM material_attribute_definitions;
DELETE FROM material_unit_conversions;
DELETE FROM materials;
DELETE FROM material_categories;
DELETE FROM units;
DELETE FROM warehouses;

COMMIT;
*/
