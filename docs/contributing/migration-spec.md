# 数据库迁移脚本规范

> 面向 CoreKG 贡献者的 MySQL 迁移脚本编写约定。归档路径：`scripts/mysql/`（另见 `scripts/mysql_saas/` 等按部署形态划分的迁移目录）。

## 基本规则

- **已发布版本的脚本除了有导致完全无法执行的语法错误外，都不允许修改**，一律通过新版本脚本实现变更或回滚
- **脚本文件仅执行一遍**
- 文件名：全小写、数字、下划线

## 文件命名

```
${version}_${seq}__${action}.sql
```

| 段 | 说明 | 示例 |
|---|---|---|
| `${version}` | 主版本号，可重复值 | `v1.6` |
| `${seq}` | 序号，可重复值 | `1`, `2`, `3` |
| `${action}` | 动作 | `create_table`, `insert_data`, `alter_table` 等 |

文件位置：`scripts/mysql/${version}_${seq}__${action}.sql`

示例：

- `scripts/mysql/v1.6_1__create_table.sql`
- `scripts/mysql/v1.6_2__insert_data.sql`
- `scripts/mysql/v1.7_1__alter_table.sql`

## 索引命名

索引统一使用以下前缀：

| 前缀 | 含义 |
|---|---|
| `uk_` | 唯一索引（Unique Key） |
| `idx_` | 普通索引（Index） |

## 常见写法示例

### 多条有外键关联的数据插入

```sql
INSERT INTO `t1` (`id`, `name`) VALUES (1, 'a');
SET @t1_id = LAST_INSERT_ID();
INSERT INTO `t2` (`id`, `name`, `t1_id`) VALUES (1, 'b', @t1_id);
```

### 用占位符 + 变量替换字段中的部分内容

```sql
INSERT INTO `t1` (`id`, `name`) VALUES (1, 'a-xxxxyyyyzzzz-b');
SET @t1_id = LAST_INSERT_ID();
UPDATE `t1` SET `name` = REPLACE(`name`, 'xxxxyyyyzzzz', @yg_VAR_NAME) WHERE `id` = @t1_id;
```
