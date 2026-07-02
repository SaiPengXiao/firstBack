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