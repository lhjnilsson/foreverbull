import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryEditor } from './QueryEditor';
import type { DataSource } from '../datasource';
import { QueryType } from '../types';

const datasource = {
  getExecutions: jest.fn().mockResolvedValue([{ label: 'spy 2020-01-01–2020-12-31 abcd1234', value: 'exec-uuid' }]),
  getMetrics: jest.fn().mockResolvedValue([
    { label: 'portfolio_value', value: 'portfolio_value' },
    { label: 'sharpe', value: 'sharpe' },
  ]),
} as unknown as DataSource;

function renderEditor(onChange = jest.fn(), onRunQuery = jest.fn()) {
  return {
    onChange,
    onRunQuery,
    ...render(
      <QueryEditor
        datasource={datasource}
        query={{ refId: 'A', queryType: QueryType.GetExecutionMetric }}
        onChange={onChange}
        onRunQuery={onRunQuery}
      />
    ),
  };
}

async function openSelect(label: string) {
  const input = await screen.findByLabelText(label);
  fireEvent.keyDown(input, { key: 'ArrowDown' });
  return input;
}

describe('QueryEditor', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('renders execution and metrics pickers', async () => {
    renderEditor();
    expect(await screen.findByLabelText('Execution')).toBeInTheDocument();
    expect(screen.getByLabelText('Metrics')).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Query Text' })).not.toBeInTheDocument();
  });

  it('writes execution.id and queryType GetExecutionMetric, not executionId', async () => {
    const { onChange, onRunQuery } = renderEditor();

    await openSelect('Execution');
    fireEvent.click(await screen.findByText('spy 2020-01-01–2020-12-31 abcd1234'));

    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const next = onChange.mock.calls[0][0];
    expect(next.queryType).toBe(QueryType.GetExecutionMetric);
    expect(next.execution).toEqual({ id: 'exec-uuid' });
    expect(next).not.toHaveProperty('executionId');
    expect(JSON.stringify(next)).not.toContain('executionId');
    expect(JSON.stringify(next)).not.toContain('"ID"');
    expect(onRunQuery).toHaveBeenCalled();
  });

  it('writes metrics as [{name}]', async () => {
    const { onChange } = renderEditor();

    await openSelect('Metrics');
    fireEvent.click(await screen.findByText('portfolio_value'));

    await waitFor(() => expect(onChange).toHaveBeenCalled());
    expect(onChange.mock.calls[0][0].metrics).toEqual([{ name: 'portfolio_value' }]);
  });
});
