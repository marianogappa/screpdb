import { useState, useEffect, useMemo } from 'react';
import { api } from '../api';

let cachedSchema = null;

export function useSchema() {
  const [schema, setSchema] = useState(cachedSchema);
  const [loading, setLoading] = useState(!cachedSchema);

  useEffect(() => {
    if (cachedSchema) return;
    let cancelled = false;
    api.getSchema()
      .then(data => {
        if (!cancelled) {
          cachedSchema = data;
          setSchema(data);
          setLoading(false);
        }
      })
      .catch(() => {
        if (!cancelled) setLoading(false);
      });
    return () => { cancelled = true; };
  }, []);

  const allColumns = useMemo(() => {
    if (!schema?.tables) return [];
    const cols = [];
    for (const [tableName, table] of Object.entries(schema.tables)) {
      for (const [colName, colInfo] of Object.entries(table.columns)) {
        cols.push({
          table: tableName,
          column: colName,
          type: colInfo.type,
          nullable: colInfo.nullable,
          qualified: `${tableName}.${colName}`,
        });
      }
    }
    return cols;
  }, [schema]);

  return { schema, loading, allColumns };
}
