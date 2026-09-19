import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { VariableQueryEditor } from './VariableQueryEditor';
import type { DataSource } from '../datasource';
import { QueryType, ResourcePath } from '../types';

const datasource = {} as DataSource;

describe('VariableQueryEditor', () => {
  it('defaults the picker to executions for $execution', async () => {
    render(
      <VariableQueryEditor
        datasource={datasource}
        query={{ refId: 'A', queryType: QueryType.GetExecutionMetric }}
        onChange={jest.fn()}
        onRunQuery={jest.fn()}
      />
    );

    expect(await screen.findByLabelText('Variable source')).toBeInTheDocument();
    expect(screen.getByText('Executions')).toBeInTheDocument();
    expect(screen.getByLabelText('Strategy filter')).toBeInTheDocument();
  });

  it('writes source executions/strategies/experiments, not executionId', async () => {
    const onChange = jest.fn();
    render(
      <VariableQueryEditor
        datasource={datasource}
        query={{ refId: 'A', queryType: QueryType.GetExecutionMetric, source: ResourcePath.Executions }}
        onChange={onChange}
        onRunQuery={jest.fn()}
      />
    );

    fireEvent.keyDown(await screen.findByLabelText('Variable source'), { key: 'ArrowDown' });
    fireEvent.click(await screen.findByText('Strategies'));

    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const [query, definition] = onChange.mock.calls[0];
    expect(query.source).toBe(ResourcePath.Strategies);
    expect(query).not.toHaveProperty('executionId');
    expect(JSON.stringify(query)).not.toContain('executionId');
    expect(definition).toBe('strategies');
  });

  it('stores cascade filters without renaming execution', async () => {
    const onChange = jest.fn();
    render(
      <VariableQueryEditor
        datasource={datasource}
        query={{ refId: 'A', queryType: QueryType.GetExecutionMetric, source: ResourcePath.Executions }}
        onChange={onChange}
        onRunQuery={jest.fn()}
      />
    );

    fireEvent.change(await screen.findByLabelText('Strategy filter'), { target: { value: '$strategy' } });

    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const [query, definition] = onChange.mock.calls[0];
    expect(query.source).toBe(ResourcePath.Executions);
    expect(query.strategy).toBe('$strategy');
    expect(query).not.toHaveProperty('executionId');
    expect(definition).toBe('executions ($strategy)');
  });
});
