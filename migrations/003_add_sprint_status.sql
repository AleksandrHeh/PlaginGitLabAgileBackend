-- Добавляем поле статуса в таблицу sprint
ALTER TABLE sprint ADD COLUMN IF NOT EXISTS spt_status VARCHAR(20) DEFAULT 'active';

-- Обновляем существующие записи
UPDATE sprint SET spt_status = 'active' WHERE spt_status IS NULL;

-- Добавляем ограничение на возможные значения статуса
ALTER TABLE sprint ADD CONSTRAINT sprint_status_check 
    CHECK (spt_status IN ('active', 'completed', 'cancelled')); 