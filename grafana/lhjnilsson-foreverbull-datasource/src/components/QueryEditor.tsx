import React, { ChangeEvent, useState } from 'react';
import { InlineField, Input, Stack, Select, AsyncMultiSelect } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { MyDataSourceOptions, MyQuery, DEFAULT_QUERY, QueryType } from '../types';
import defaults from 'lodash/defaults';

export interface QueryTypeInfo extends SelectableValue<QueryType> {
  value: QueryType;
}

export const queryTypeInfos: QueryTypeInfo[] = [
  {
    label: 'Get Execution Metric',
    value: QueryType.GetExecutionMetric,
    description: ``,
  },
];

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor({ datasource, query, onChange, onRunQuery }: Props) {
  const [queryType, setQueryType] = useState(query.queryType);
  const [q, setQuery] = useState(defaults(query, DEFAULT_QUERY));

  const updateAndRunQuery = (q: MyQuery) => {
    onChange(q);
    setQuery(q);
    onRunQuery();
  };

  const loadExecutions = (): Promise<Array<SelectableValue<string>>> => {
    return datasource.getExecutions();
  };

  const loadMetrics = (): Promise<Array<SelectableValue<string>>> => {
    return datasource.getMetrics();
  };

  const selectedExecutions = q.executionIds?.map((x) => ({ label: x.label, value: x.value }));
  const selectedMetrics = q.metrics?.map((x) => ({ label: x.label, value: x.value }));

  const onExecutionChange = (evt: Array<SelectableValue<string>>) => {
    const m = evt.map((x) => ({ value: x.value, label: x.label }));
    updateAndRunQuery({ ...q, executionIds: m });
  };
  const onMetricChange = (evt: Array<SelectableValue<string>>) => {
    const m = evt.map((x) => ({ value: x.value, label: x.label }));
    updateAndRunQuery({ ...q, metrics: m });
  };

  const executionKey = q.executionIds?.map((x) => x.key).join();
  // END

  const currentQueryType = queryTypeInfos.find((v) => v.value === query.queryType);
  const onQueryTypeChange = async (queryType: QueryType) => {
    setQueryType(queryType);
    updateAndRunQuery({ ...query, queryType: queryType });
  };

  return (
    <Stack gap={0}>
      <Select
        defaultValue={queryTypeInfos[0]}
        options={queryTypeInfos}
        value={currentQueryType}
        onChange={(x) => onQueryTypeChange(x.value || QueryType.GetExecutionMetric)}
        width={32}
        required={true}
      />
      <AsyncMultiSelect
        width={96}
        defaultOptions={true}
        key={executionKey}
        value={selectedExecutions}
        loadOptions={loadExecutions}
        onChange={(evt) => onExecutionChange(evt)}
        allowCustomValue={true}
        isSearchable={true}
      ></AsyncMultiSelect>
      <AsyncMultiSelect
        width={96}
        defaultOptions={true}
        value={selectedMetrics}
        loadOptions={loadMetrics}
        onChange={(evt) => onMetricChange(evt)}
        allowCustomValue={true}
        isSearchable={true}
      ></AsyncMultiSelect>
    </Stack>
  );
}
