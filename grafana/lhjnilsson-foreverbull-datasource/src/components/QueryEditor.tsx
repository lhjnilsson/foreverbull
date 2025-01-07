import React, { ChangeEvent, useState } from 'react';
import { InlineField, Input, Stack, Select, AsyncMultiSelect, AsyncSelect, SelectValue, QueryField } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { MyDataSourceOptions, MyQuery, ResourceDefinition } from '../types';
import defaults from 'lodash/defaults';

export interface QueryTypeInfo extends SelectableValue<ResourceDefinition> {
  value: ResourceDefinition;
}

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor({ datasource, query, onChange, onRunQuery }: Props) {
  const [selectedExecution, setSelectedExecution] = useState<SelectableValue<string> | null>(null);

  const loadExecutions = (): Promise<Array<SelectableValue<string>>> => {
    return datasource.getExecutions();
  };

  const onExecutionChange = async (queryType: SelectableValue<string>) => {
    setSelectedExecution(queryType);
    onChange({ ...query, executionId: queryType.value });
    onRunQuery();
  };

  const onQueryTextChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, executionId: event.target.value });
    onRunQuery();
  };

  const { executionId } = query;

  return (
    <Stack gap={0}>
      <AsyncSelect
        loadOptions={loadExecutions}
        value={selectedExecution}
        defaultOptions={true}
        onChange={(evt) => onExecutionChange(evt)}
        width={32}
        required={false}
        allowCustomValue={true}
      />
      <InlineField label="Execution">
        <Input
          id="demodemo"
          onChange={onQueryTextChange}
          value={executionId || ''}
          required
          placeholder="Enter a query"
        />
      </InlineField>
    </Stack>
  );
}
