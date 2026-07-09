-- RBAC：角色 + 权限表（灵活权限管理）
-- 在线上 Railway 的 railway 库上执行即可，全部幂等（IF NOT EXISTS / INSERT IGNORE）。
-- DSN 已带 multiStatements=true，可整段粘贴运行。

USE railway;

-- 角色
CREATE TABLE IF NOT EXISTS roles (
  id CHAR(36) PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  description VARCHAR(255) DEFAULT NULL,
  created_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_roles_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 权限（资源:动作，如 menu:category:create）
CREATE TABLE IF NOT EXISTS permissions (
  id CHAR(36) PRIMARY KEY,
  code VARCHAR(64) NOT NULL,
  description VARCHAR(255) DEFAULT NULL,
  created_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_permissions_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 角色 ↔ 权限
CREATE TABLE IF NOT EXISTS role_permissions (
  role_id CHAR(36) NOT NULL,
  permission_id CHAR(36) NOT NULL,
  PRIMARY KEY (role_id, permission_id),
  CONSTRAINT fk_rp_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_rp_perm FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 用户 ↔ 角色（一个用户可拥有多个角色，权限取并集）
CREATE TABLE IF NOT EXISTS user_roles (
  user_id CHAR(36) NOT NULL,
  role_id CHAR(36) NOT NULL,
  PRIMARY KEY (user_id, role_id),
  CONSTRAINT fk_ur_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_ur_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 权限种子（固定 UUID，便于幂等）
INSERT IGNORE INTO permissions (id, code, description, created_at) VALUES
  ('33333333-0000-0000-0000-000000000001', 'menu:read',            '查看菜单',           NOW(3)),
  ('33333333-0000-0000-0000-000000000002', 'menu:category:create', '新建菜单分类',       NOW(3)),
  ('33333333-0000-0000-0000-000000000003', 'menu:category:update', '修改菜单分类',       NOW(3)),
  ('33333333-0000-0000-0000-000000000004', 'menu:category:delete', '删除菜单分类',       NOW(3)),
  ('33333333-0000-0000-0000-000000000005', 'menu:item:create',     '新建菜品',           NOW(3)),
  ('33333333-0000-0000-0000-000000000006', 'menu:item:update',     '修改菜品',           NOW(3)),
  ('33333333-0000-0000-0000-000000000007', 'menu:item:delete',     '删除菜品',           NOW(3)),
  ('33333333-0000-0000-0000-000000000008', 'order:read',           '查看订单',           NOW(3));

-- 角色种子：admin 拥有菜单全部读写权限
INSERT IGNORE INTO roles (id, name, description, created_at) VALUES
  ('44444444-0000-0000-0000-000000000001', 'admin', '管理员', NOW(3));

INSERT IGNORE INTO role_permissions (role_id, permission_id)
  SELECT r.id, p.id FROM roles r, permissions p
  WHERE r.name = 'admin';

-- ===== 手动把某用户设为管理员（替换用户名后执行）=====
-- INSERT INTO user_roles (user_id, role_id)
--   SELECT u.id, r.id FROM users u, roles r
--   WHERE u.username = '替换成你的用户名' AND r.name = 'admin';
