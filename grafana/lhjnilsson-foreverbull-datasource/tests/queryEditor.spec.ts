import { test, expect } from '@grafana/plugin-e2e';

test('smoke: should render query editor', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  const row = panelEditPage.getQueryEditorRow('A');
  await expect(row.getByLabel('Execution')).toBeVisible();
  await expect(row.getByLabel('Metrics')).toBeVisible();
  await expect(row.getByRole('textbox', { name: 'Query Text' })).toHaveCount(0);
});
