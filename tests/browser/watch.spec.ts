import { expect, test } from '@playwright/test';

test('renders the 240px watch fixture and completes a task', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByTestId('watch-face')).toBeVisible();
  await expect(page.getByText('9:41 am')).toBeVisible();
  await expect(page.getByText('19', { exact: true })).toBeVisible();
  await page.getByTestId('watch-face').screenshot({ path: 'test-results/watch-face.png' });
  await page.getByRole('button', { name: 'Complete selected task' }).click();
  await expect(page.getByRole('button', { name: 'Refine the website' }).first()).toHaveClass(/done/);
});

test('adds a task through the dashboard', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: /add task/ }).click();
  await page.getByLabel('Task name').fill('Write launch note');
  await page.getByLabel('Duration in minutes').fill('12');
  await page.getByRole('button', { name: 'add', exact: true }).click();
  await expect(page.getByText('Write launch note').first()).toBeVisible();
});
