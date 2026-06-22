INSERT INTO users (role_id, email, password_hash, full_name)
VALUES (
           (SELECT id FROM roles WHERE name = 'doctor'),
           'doctor@dentvision.com',
           '$2y$10$lyM5loAX20J46kKCM/gmvOewWZER4nNQcXPqnj.oIxBmPlkFiyc/i',
           'Тестовый врач'
       );

INSERT INTO analysis_models (name, version, description, is_active)
VALUES (
           'DentVision Demo Model',
           '0.1.0',
           'Тестовая модель для предварительного анализа стоматологических снимков',
           TRUE
       );