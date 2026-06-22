CREATE TABLE roles
(
    id    BIGSERIAL PRIMARY KEY,
    name  VARCHAR(50)  NOT NULL UNIQUE,
    title VARCHAR(100) NOT NULL
);

INSERT INTO roles (name, title)
VALUES ('doctor', 'Врач'),
       ('admin', 'Администратор');

CREATE TABLE users
(
    id            BIGSERIAL PRIMARY KEY,
    role_id       BIGINT       NOT NULL REFERENCES roles (id),
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT         NOT NULL,
    full_name     VARCHAR(255) NOT NULL,
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE TABLE patients
(
    id         BIGSERIAL PRIMARY KEY,
    doctor_id  BIGINT       NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    full_name  VARCHAR(255) NOT NULL,
    birth_date DATE,
    phone      VARCHAR(50),
    comment    TEXT,
    created_at TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE TABLE image_files
(
    id            BIGSERIAL PRIMARY KEY,
    file_path     TEXT         NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    mime_type     VARCHAR(100) NOT NULL,
    file_size     BIGINT,
    created_at    TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE TABLE dental_images
(
    id          BIGSERIAL PRIMARY KEY,
    patient_id  BIGINT      NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    file_id     BIGINT      NOT NULL REFERENCES image_files (id) ON DELETE CASCADE,
    uploaded_by BIGINT      NOT NULL REFERENCES users (id),
    image_type  VARCHAR(100)         DEFAULT 'xray',
    status      VARCHAR(50) NOT NULL DEFAULT 'uploaded',
    created_at  TIMESTAMP   NOT NULL DEFAULT NOW()
);

CREATE TABLE analysis_models
(
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    version     VARCHAR(100) NOT NULL,
    description TEXT,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE TABLE analysis_jobs
(
    id            BIGSERIAL PRIMARY KEY,
    image_id      BIGINT      NOT NULL REFERENCES dental_images (id) ON DELETE CASCADE,
    model_id      BIGINT REFERENCES analysis_models (id),
    status        VARCHAR(50) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    started_at    TIMESTAMP,
    finished_at   TIMESTAMP,
    created_at    TIMESTAMP   NOT NULL DEFAULT NOW()
);

CREATE TABLE analysis_results
(
    id         BIGSERIAL PRIMARY KEY,
    job_id     BIGINT        NOT NULL REFERENCES analysis_jobs (id) ON DELETE CASCADE,
    image_id   BIGINT        NOT NULL REFERENCES dental_images (id) ON DELETE CASCADE,
    label      VARCHAR(100)  NOT NULL,
    confidence NUMERIC(5, 4) NOT NULL,
    x          INTEGER       NOT NULL,
    y          INTEGER       NOT NULL,
    width      INTEGER       NOT NULL,
    height     INTEGER       NOT NULL,
    created_at TIMESTAMP     NOT NULL DEFAULT NOW()
);

CREATE TABLE doctor_feedback
(
    id           BIGSERIAL PRIMARY KEY,
    result_id    BIGINT    NOT NULL REFERENCES analysis_results (id) ON DELETE CASCADE,
    doctor_id    BIGINT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    is_confirmed BOOLEAN,
    comment      TEXT,
    created_at   TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE audit_logs
(
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT       REFERENCES users (id) ON DELETE SET NULL,
    action      VARCHAR(100) NOT NULL,
    entity_type VARCHAR(100),
    entity_id   BIGINT,
    created_at  TIMESTAMP    NOT NULL DEFAULT NOW()
);