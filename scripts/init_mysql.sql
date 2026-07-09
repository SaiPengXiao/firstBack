-- 在 Navicat：选中连接「sql联系」→ 新建查询 → 粘贴全部 → 运行
-- 库名可按需改成 back_system 等，同时改环境变量 MYSQL_DATABASE

CREATE DATABASE IF NOT EXISTS firstback
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

USE firstback;

-- 用户表（与后端 migrate 一致）
CREATE TABLE IF NOT EXISTS users (
  id CHAR(36) PRIMARY KEY,
  username VARCHAR(191) NOT NULL,
  email VARCHAR(191) NOT NULL,
  password_hash VARBINARY(255) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_users_username (username),
  UNIQUE KEY uk_users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 菜单分类（如：热菜、凉菜、饮品）
CREATE TABLE IF NOT EXISTS menu_categories (
  id CHAR(36) PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_menu_categories_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 点菜菜品
CREATE TABLE IF NOT EXISTS menu_items (
  id CHAR(36) PRIMARY KEY,
  category_id CHAR(36) NOT NULL,
  name VARCHAR(128) NOT NULL,
  description TEXT,
  price DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  image_url VARCHAR(512) DEFAULT NULL,
  is_available TINYINT(1) NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  KEY idx_menu_items_category (category_id),
  KEY idx_menu_items_available (is_available),
  CONSTRAINT fk_menu_items_category FOREIGN KEY (category_id) REFERENCES menu_categories(id) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 可选：示例分类和菜品（不需要可删掉下面 INSERT）
INSERT INTO menu_categories (id, name, sort_order, created_at) VALUES
  ('11111111-1111-1111-1111-111111111101', '热菜', 1, NOW(3)),
  ('11111111-1111-1111-1111-111111111102', '饮品', 2, NOW(3))
ON DUPLICATE KEY UPDATE name = VALUES(name);

INSERT INTO menu_items (id, category_id, name, description, price, is_available, sort_order, created_at, updated_at) VALUES
  ('22222222-2222-2222-2222-222222222201', '11111111-1111-1111-1111-111111111101', '宫保鸡丁', '经典川菜', 38.00, 1, 1, NOW(3), NOW(3)),
  ('22222222-2222-2222-2222-222222222202', '11111111-1111-1111-1111-111111111102', '柠檬水', '冰镇', 12.00, 1, 1, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE name = VALUES(name);

-- ========== RBAC：角色 + 权限表 ==========
CREATE TABLE IF NOT EXISTS roles (
  id CHAR(36) PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  description VARCHAR(255) DEFAULT NULL,
  created_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_roles_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS permissions (
  id CHAR(36) PRIMARY KEY,
  code VARCHAR(64) NOT NULL,
  description VARCHAR(255) DEFAULT NULL,
  created_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_permissions_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS role_permissions (
  role_id CHAR(36) NOT NULL,
  permission_id CHAR(36) NOT NULL,
  PRIMARY KEY (role_id, permission_id),
  CONSTRAINT fk_rp_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_rp_perm FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_roles (
  user_id CHAR(36) NOT NULL,
  role_id CHAR(36) NOT NULL,
  PRIMARY KEY (user_id, role_id),
  CONSTRAINT fk_ur_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_ur_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT IGNORE INTO permissions (id, code, description, created_at) VALUES
  ('33333333-0000-0000-0000-000000000001', 'menu:read',            '查看菜单',           NOW(3)),
  ('33333333-0000-0000-0000-000000000002', 'menu:category:create', '新建菜单分类',       NOW(3)),
  ('33333333-0000-0000-0000-000000000003', 'menu:category:update', '修改菜单分类',       NOW(3)),
  ('33333333-0000-0000-0000-000000000004', 'menu:category:delete', '删除菜单分类',       NOW(3)),
  ('33333333-0000-0000-0000-000000000005', 'menu:item:create',     '新建菜品',           NOW(3)),
  ('33333333-0000-0000-0000-000000000006', 'menu:item:update',     '修改菜品',           NOW(3)),
  ('33333333-0000-0000-0000-000000000007', 'menu:item:delete',     '删除菜品',           NOW(3)),
  ('33333333-0000-0000-0000-000000000008', 'order:read',           '查看订单',           NOW(3));

INSERT IGNORE INTO roles (id, name, description, created_at) VALUES
  ('44444444-0000-0000-0000-000000000001', 'admin', '管理员', NOW(3));

INSERT IGNORE INTO role_permissions (role_id, permission_id)
  SELECT r.id, p.id FROM roles r, permissions p
  WHERE r.name = 'admin';

-- 手动把某用户设为管理员（替换用户名后执行）：
-- INSERT INTO user_roles (user_id, role_id)
--   SELECT u.id, r.id FROM users u, roles r
--   WHERE u.username = '替换成你的用户名' AND r.name = 'admin';