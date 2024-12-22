import React, { ChangeEvent, useState } from 'react';
import { InlineField, Input, Stack } from '@grafana/ui';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from '../datasource';
import { MyDataSourceOptions, MyQuery, DEFAULT_QUERY } from '../types';

/*
MYSTUFF
*/
import { SelectableValue } from '@grafana/data';
import { Select } from '@grafana/ui';
import defaults from 'lodash/defaults';

export enum QueryType {
  GetMetricValue = 'GetMetricValue',
  GetMetricHistory = 'GetMetricHistory',
  GetMetricAggregate = 'GetMetricAggregate',
}

export interface QueryTypeInfo extends SelectableValue<QueryType> {
  value: QueryType; // not optional
}

export const queryTypeInfos: QueryTypeInfo[] = [
  {
    label: 'Get metric history',
    value: QueryType.GetMetricHistory,
    description: `Gets the history of a metric.`,
  },
  {
    label: 'Get metric value',
    value: QueryType.GetMetricValue,
    description: `Gets a metrics current value.`,
  },
  {
    label: 'Get metric aggregate',
    value: QueryType.GetMetricAggregate,
    description: `Gets a metrics aggregate value.`,
  },
];

/*
END
*/

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
  const { queryText, constant } = query;

  // MY
  const [queryType, setQueryType] = useState(query.queryType);
  const [_query, setQuery] = useState(defaults(query, DEFAULT_QUERY));

  const updateAndRunQuery = (q: MyQuery) => {
    onChange(q);
    setQuery(q);
    onRunQuery();
  };

  const onQueryTypeChange = async (queryType: QueryType) => {
    setQueryType(queryType);
    updateAndRunQuery({ ...query, queryType: queryType, queryOptions: {} });
  };
  const currentQueryType = queryTypeInfos.find((v) => v.value === query.queryType);

  // END

  return (
    <Stack gap={0}>
      <InlineField labelWidth={24} label="Query Type">
        <Select
          options={queryTypeInfos}
          value={currentQueryType}
          onChange={(x) => onQueryTypeChange(x.value || QueryType.GetMetricAggregate)}
          width={32}
        />
      </InlineField>
      {queryType === QueryType.GetMetricValue && (
        <Input placeholder="Temperature threshold" type="number" onChange={() => console.log('hello')} />
      )}
    </Stack>
  );
}
