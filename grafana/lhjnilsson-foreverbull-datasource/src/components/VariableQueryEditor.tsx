import React, { ChangeEvent } from 'react';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { InlineField, InlineFieldRow, Input, Select } from '@grafana/ui';
import type { DataSource } from '../datasource';
import { variableDefinition, variableSource } from '../query';
import { MyDataSourceOptions, MyQuery, ResourcePath, VariableSource } from '../types';

const SOURCE_OPTIONS: Array<SelectableValue<VariableSource>> = [
  { label: 'Executions', value: ResourcePath.Executions, description: 'Populates $execution with execution UUIDs' },
  { label: 'Strategies', value: ResourcePath.Strategies, description: 'Populates $strategy with strategy names' },
  { label: 'Versions', value: ResourcePath.Versions, description: 'Populates $version with version ids' },
  { label: 'Experiments', value: ResourcePath.Experiments, description: 'Populates $experiment; values are execution UUIDs' },
];

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;
type OnVariableQueryChange = (query: MyQuery, definition?: string) => void;

export function VariableQueryEditor({ query, onChange }: Props) {
  const source = variableSource(query);
  const setQuery = onChange as OnVariableQueryChange;

  const update = (patch: Partial<MyQuery>) => {
    const next: MyQuery = { ...query, ...patch };
    setQuery(next, variableDefinition(next));
  };

  const onSourceChange = (evt: SelectableValue<VariableSource>) => {
    if (!evt.value) {
      return;
    }
    update({ source: evt.value });
  };

  const onFilterChange = (key: 'strategy' | 'version' | 'backtest') => (event: ChangeEvent<HTMLInputElement>) => {
    update({ [key]: event.currentTarget.value });
  };

  const showFilters = source !== ResourcePath.Strategies;

  return (
    <>
      <InlineFieldRow>
        <InlineField
          label="Query"
          tooltip="Lists values for a dashboard variable. Executions and experiments use execution UUIDs so $execution deep links keep working."
        >
          <Select
            inputId="variable-query-source"
            aria-label="Variable source"
            options={SOURCE_OPTIONS}
            value={SOURCE_OPTIONS.find((option) => option.value === source)}
            onChange={onSourceChange}
            width={32}
          />
        </InlineField>
      </InlineFieldRow>
      {showFilters && (
        <InlineFieldRow>
          <InlineField label="Strategy">
            <Input
              id="variable-query-strategy"
              aria-label="Strategy filter"
              value={query.strategy ?? ''}
              placeholder="$strategy"
              onChange={onFilterChange('strategy')}
              width={20}
            />
          </InlineField>
          {source !== ResourcePath.Versions && (
            <InlineField label="Version">
              <Input
                id="variable-query-version"
                aria-label="Version filter"
                value={query.version ?? ''}
                placeholder="$version"
                onChange={onFilterChange('version')}
                width={20}
              />
            </InlineField>
          )}
          <InlineField label="Backtest">
            <Input
              id="variable-query-backtest"
              aria-label="Backtest filter"
              value={query.backtest ?? ''}
              placeholder="$backtest"
              onChange={onFilterChange('backtest')}
              width={20}
            />
          </InlineField>
        </InlineFieldRow>
      )}
    </>
  );
}

export default VariableQueryEditor;
