DO $$ 
BEGIN
    -- Проверяем существование столбца spt_status
    IF NOT EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name = 'sprint' 
        AND column_name = 'spt_status'
    ) THEN
        -- Добавляем столбец, если его нет
        ALTER TABLE sprint ADD COLUMN spt_status VARCHAR(20) DEFAULT 'active';
        
        -- Обновляем существующие записи
        UPDATE sprint SET spt_status = 'active' WHERE spt_status IS NULL;
    END IF;

    -- Проверяем существование ограничения sprint_status_check
    IF NOT EXISTS (
        SELECT 1 
        FROM information_schema.table_constraints 
        WHERE table_name = 'sprint' 
        AND constraint_name = 'sprint_status_check'
    ) THEN
        -- Добавляем ограничение, если его нет
        ALTER TABLE sprint ADD CONSTRAINT sprint_status_check 
            CHECK (spt_status IN ('active', 'completed', 'cancelled'));
    END IF;
END $$; 