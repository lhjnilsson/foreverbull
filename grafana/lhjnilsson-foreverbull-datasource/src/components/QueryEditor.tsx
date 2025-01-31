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
  const loadExecutions = (): Promise<Array<SelectableValue<string>>> => {
    return datasource.getExecutions();
  };

  const loadMetrics = (): Promise<Array<SelectableValue<string>>> => {
    return datasource.getMetrics();
  };

  const onExecutionChange = async (execution: SelectableValue<string>) => {
    onChange({ ...query, execution: { ID: execution.value } });
    onRunQuery();
  };

  const onMetricChange = async (evt: Array<SelectableValue<string>>) => {
    const m = evt.map((x) => ({ name: x.value }));
    onChange({ ...query, metrics: m });
    onRunQuery();
  };

  const onQueryTextChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, execution: { ID: event.target.value } });
    onRunQuery();
  };

  const selectedMetrics = query.metrics?.map((x) => ({ label: x.name, value: x.name }));
  const selectedExecution = query.execution ? { label: query.execution.ID, value: query.execution.ID } : null;

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
      <AsyncMultiSelect
        loadOptions={loadMetrics}
        value={selectedMetrics}
        defaultOptions={true}
        onChange={(evt) => onMetricChange(evt)}
        width={32}
        required={false}
        allowCustomValue={true}
      />
      <InlineField label="Execution">
        <Input
          onChange={onQueryTextChange}
          value={selectedExecution ? selectedExecution.value : ''}
          required
          placeholder="Enter a query"
        />
      </InlineField>
    </Stack>
  );
}
