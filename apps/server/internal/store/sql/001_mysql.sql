-- kingdee-rpt 系统库（MySQL 8.0）
-- GORM AutoMigrate 会自动建表，此文件为参考/手工建库用。
CREATE DATABASE IF NOT EXISTS kingdee_rpt DEFAULT CHARACTER SET utf8mb4;
USE kingdee_rpt;

CREATE TABLE IF NOT EXISTS rpt_instance (
  id           BIGINT AUTO_INCREMENT PRIMARY KEY,
  report_code  VARCHAR(64) NOT NULL,
  tpl_version  INT         NOT NULL,
  params       JSON        NOT NULL,
  params_hash  CHAR(64)    NOT NULL,
  status       TINYINT     NOT NULL DEFAULT 0,
  data         JSON        NOT NULL,
  updated_at   DATETIME(3) NOT NULL,
  updated_by   VARCHAR(64) NULL,
  submitted_at DATETIME    NULL,
  UNIQUE KEY uk_code_params (report_code, params_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS rpt_submission (
  id          BIGINT AUTO_INCREMENT PRIMARY KEY,
  instance_id BIGINT   NOT NULL,
  action      VARCHAR(16) NOT NULL,
  snapshot    JSON      NOT NULL,
  op          VARCHAR(64) NULL,
  created_at  DATETIME  NOT NULL,
  KEY idx_inst (instance_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS rpt_import_job (
  id          BIGINT AUTO_INCREMENT PRIMARY KEY,
  instance_id BIGINT   NOT NULL,
  file_name   VARCHAR(255) NULL,
  file_sha256 CHAR(64) NULL,
  meta_tpl    VARCHAR(64) NULL,
  meta_ver    INT NULL,
  meta_params JSON NULL,
  data        JSON NULL,
  status      VARCHAR(16) NOT NULL,
  error_rpt   JSON NULL,
  diff_sum    JSON NULL,
  op          VARCHAR(64) NULL,
  created_at  DATETIME NOT NULL,
  KEY idx_inst (instance_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS rpt_template (
  code       VARCHAR(64) NOT NULL,
  version    INT         NOT NULL,
  yaml       TEXT        NOT NULL,
  checksum   CHAR(64)    NOT NULL,
  updated_at DATETIME    NOT NULL,
  PRIMARY KEY (code, version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
