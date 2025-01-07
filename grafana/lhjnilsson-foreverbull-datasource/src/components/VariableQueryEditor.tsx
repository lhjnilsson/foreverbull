import React, { useState } from 'react';
import { MyQuery } from '../types';
import { DataSource } from '../datasource';
import { SelectableValue } from '@grafana/data';
import { AsyncSelect, InlineField, Input, Select, InlineFieldRow } from '@grafana/ui';

const VariableQueryEditor = (props: {
  query: MyQuery;
  onChange: (query: MyQuery, definition: string) => void;
  datasource: DataSource;
}) => {
  const dims: any[] = [];
  const { datasource, onChange, query } = props;
  const [selectedExecution, setSelectedExecution] = useState<SelectableValue<string> | null>(null);

  const loadExecutions = (): Promise<Array<SelectableValue<string>>> => {
    return datasource.getExecutions();
  };

  const onExecutionChange = async (event: SelectableValue<string>) => {
    setSelectedExecution(event);
    onChange({ ...query, executionId: event.label }, event.label);
  };

  return (
    <AsyncSelect
      loadOptions={loadExecutions}
      value={selectedExecution}
      defaultOptions={true}
      onChange={(evt) => onExecutionChange(evt)}
      width={32}
      required={false}
    />
  );
};

export default VariableQueryEditor;
