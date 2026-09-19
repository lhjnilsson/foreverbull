import React, { useState } from 'react';
import { AsyncMultiSelect, AsyncSelect, InlineField, InlineFieldRow } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import type { DataSource } from '../datasource';
import { executionRefId, toExecution, toMetrics, withExecutionMetric } from '../query';
import { MyDataSourceOptions, MyQuery } from '../types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor({ datasource, query, onChange, onRunQuery }: Props) {
  const [executionOptions, setExecutionOptions] = useState<Array<SelectableValue<string>>>([]);

  const loadExecutions = async (): Promise<Array<SelectableValue<string>>> => {
    const items = await datasource.getExecutions();
    setExecutionOptions(items);
    return items;
  };

  const loadMetrics = (): Promise<Array<SelectableValue<string>>> => {
    return datasource.getMetrics();
  };

  const onExecutionChange = (execution: SelectableValue<string> | null) => {
    onChange(withExecutionMetric(query, { execution: toExecution(execution?.value) }));
    onRunQuery();
  };

  const onMetricChange = (evt: Array<SelectableValue<string>>) => {
    onChange(withExecutionMetric(query, { metrics: toMetrics(evt.map((x) => x.value)) }));
    onRunQuery();
  };

  const selectedId = executionRefId(query.execution);
  const selectedExecution = selectedId
    ? executionOptions.find((item) => item.value === selectedId) ?? { label: selectedId, value: selectedId }
    : null;
  const selectedMetrics = query.metrics?.map((metric) => ({ label: metric.name, value: metric.name }));

  return (
    <InlineFieldRow>
      <InlineField
        label="Execution"
        tooltip="Execution UUID. Use $execution for the dashboard variable. Overlay another run as query B."
      >
        <AsyncSelect
          inputId="query-editor-execution"
          aria-label="Execution"
          loadOptions={loadExecutions}
          value={selectedExecution}
          defaultOptions={true}
          onChange={onExecutionChange}
          width={40}
          allowCustomValue={true}
          placeholder="Select execution or $execution"
        />
      </InlineField>
      <InlineField label="Metrics" tooltip="Period columns from GetExecution. Overlay is a second GetExecutionMetric query.">
        <AsyncMultiSelect
          inputId="query-editor-metrics"
          aria-label="Metrics"
          loadOptions={loadMetrics}
          value={selectedMetrics}
          defaultOptions={true}
          onChange={onMetricChange}
          width={40}
          allowCustomValue={true}
          placeholder="Select metrics"
        />
      </InlineField>
    </InlineFieldRow>
  );
}
