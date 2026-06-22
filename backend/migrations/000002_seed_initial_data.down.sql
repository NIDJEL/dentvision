DELETE FROM analysis_models
WHERE name = 'DentVision Demo Model'
  AND version = '0.1.0';

DELETE FROM users
WHERE email = 'doctor@dentvision.com';