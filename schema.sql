-- schema.sql —— users 表的建表语句
--
-- 执行后 PostgreSQL 会创建 users 表。
--
-- 这个文件存在的意义：让项目可复现。
--
-- 在此之前，users 表只存在于作者本机的数据库里。任何人 clone 这个仓库
-- （包括换一台电脑的作者自己）都跑不起来，因为没人知道表长什么样。
--
-- 执行方式见 README.md。
--
-- 注意一个陷阱：CREATE TABLE IF NOT EXISTS 是幂等的（重复执行不报错），
-- 但如果你本地已经有一个结构不确定的 users 表，它会静默跳过，
-- 导致这个文件和实际表结构不一致 —— 而这恰好破坏了"可复现"这个目标。
-- 如果不确定，先执行下面这行清掉旧表，再执行建表语句：
--
--     DROP TABLE IF EXISTS users;

CREATE TABLE IF NOT EXISTS users (
    -- SERIAL = 自增整数（插入时不指定，由数据库分配下一个值）
    -- PRIMARY KEY = 主键，唯一标识一行，数据库会自动为它建索引
    --
    -- 为什么必须有：GET / PUT / DELETE /users/:id 三个接口全靠它定位记录，
    -- user_service.go 里的 WHERE id = $1 直接依赖它。
    --
    -- 为什么不用 UUID：Phase 1 不需要。代价是自增 id 可被顺序枚举
    -- （别人能靠 /users/1、/users/2 遍历全部用户），但那是有了认证之后
    -- 才需要权衡的事，现在引入 UUID 属于提前增加复杂度。
    id       SERIAL PRIMARY KEY,

    -- NOT NULL 不是洁癖，是代码的硬要求：
    -- router.go 里 rows.Scan(&user.Username) 要把这一列扫进 Go 的 string，
    -- 而 Go 的 string 无法表示 NULL，遇到 NULL 会直接返回错误，
    -- 导致 GET /users 返回 500。
    --
    -- 为什么用 TEXT 而不是 VARCHAR(n)：在 PostgreSQL 里两者的存储和性能
    -- 完全相同（这一点和 MySQL 不一样）。而长度限制放在应用层
    -- （Phase 2 的 binding 标签）改起来不需要 ALTER TABLE，还能返回
    -- 友好的错误信息。所以长度归应用管，数据库只负责"不能为空"。
    -- UNIQUE：同一个用户名只能有一行。重复插入会得到 PostgreSQL 错误码 23505，
    -- Repository 把它变成 ErrUsernameTaken，Handler 再返回 409。
    -- 已有的表不会被上面的 IF NOT EXISTS 改掉。本机旧表需要额外执行一次：
    --     ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username);
    username TEXT   NOT NULL UNIQUE,

    -- 存 bcrypt 哈希，不是用户输入的明文。
    -- 应用层写入前会哈希；GET 接口不会把这一列返回给客户端。
    -- 从 Phase 1 的 password 列改名而来：已有表不会被 IF NOT EXISTS 更新，
    -- 需要 DROP TABLE IF EXISTS users; 后再执行本文件。
    password_hash TEXT NOT NULL,

    -- 同样的 NULL 理由：Go 的 int 接不了 NULL。
    age      INT    NOT NULL
);

-- ============================================================
-- 这里特意没有加的东西，以及为什么
-- ============================================================
--
-- created_at / updated_at
--
--    当前代码没有任何地方读或写它们，加了就是死字段 —— 它会出现在表结构里、
--    出现在 SELECT * 里、出现在你向别人解释这张表的时候，但什么用都没有。
--
--    不要为"以后可能需要"提前加字段：加字段的成本比删字段低得多。
