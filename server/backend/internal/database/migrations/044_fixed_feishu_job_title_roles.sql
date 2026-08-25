UPDATE logmaster_api.users
SET role = CASE
    WHEN job_title LIKE '%主任%' THEN 'super_admin'
    WHEN job_title LIKE '%高级%' THEN 'admin'
    WHEN job_title LIKE '%软件工程师%' OR job_title LIKE '%硬件工程师%' THEN 'developer'
    ELSE 'user'
END,
updated_at = NOW()
WHERE role_source = 'feishu';
