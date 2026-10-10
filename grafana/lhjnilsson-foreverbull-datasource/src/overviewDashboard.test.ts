import fs from 'fs';
import path from 'path';

interface DashboardLink {
  url?: string;
  title?: string;
}

interface DashboardTarget {
  queryType?: string;
  backtest?: string;
  execution?: { id?: string };
}

interface DashboardPanel {
  title?: string;
  type?: string;
  targets?: DashboardTarget[];
}

interface DashboardFile {
  uid?: string;
  title?: string;
  time?: { from?: string; to?: string };
  links?: DashboardLink[];
  panels?: DashboardPanel[];
  templating?: { list?: Array<{ name?: string; allValue?: string; includeAll?: boolean }> };
}

function readDashboard(name: string): { raw: string; json: DashboardFile } {
  const file = path.join(__dirname, '..', 'provisioning', 'dashboards', name);
  const raw = fs.readFileSync(file, 'utf8');
  return { raw, json: JSON.parse(raw) as DashboardFile };
}

describe('backtest overview dashboard', () => {
  const overview = readDashboard('backtest-overview.json');
  const execution = readDashboard('executions.json');
  const compare = readDashboard('strategy-compare.json');

  it('is a separate board with a two-year window', () => {
    expect(overview.json.uid).toBe('fb-backtest-overview');
    expect(overview.json.title).toBe('Backtest overview');
    expect(overview.json.time).toEqual({ from: 'now-2y', to: 'now' });
    expect(overview.raw).not.toContain('now-30y');
    expect(overview.raw).not.toContain('executionId');
    expect(overview.json.uid).not.toBe(execution.json.uid);
    expect(overview.json.uid).not.toBe(compare.json.uid);
    expect(execution.json.uid).toBe('ee9ep4jxtwkcgf');
    expect(compare.json.uid).toBe('fb-strategy-compare');
  });

  it('lists overlapping runs with ListExecutionOverview and links each row back', () => {
    expect(overview.raw).toContain('ListExecutionOverview');
    expect(overview.raw).toContain('/d/ee9ep4jxtwkcgf?var-execution=${__data.fields.execution}');
    expect(overview.raw).toContain('var-backtest=${__data.fields.backtest}');
    expect(overview.raw).toContain('percentunit');
    expect(overview.raw).toContain('currencyUSD');
    const targets = (overview.json.panels ?? []).flatMap((panel) => panel.targets ?? []);
    expect(targets.length).toBeGreaterThan(0);
    for (const target of targets) {
      expect(target.queryType).toBe('ListExecutionOverview');
      expect(target.backtest).toBe('$backtest');
      expect(target.execution).toBeUndefined();
    }
    const backtest = (overview.json.templating?.list ?? []).find((variable) => variable.name === 'backtest');
    expect(backtest?.includeAll).toBe(true);
    expect(backtest?.allValue).toBe('*');
  });

  it('keeps the execution page link to this board', () => {
    const link = (execution.json.links ?? []).find((item) => item.url === '/d/fb-backtest-overview');
    expect(link?.title).toBe('Backtest overview');
    expect(execution.raw).toContain('ee9ep4jxtwkcgf');
  });
});