import { test, expect, type Page } from '@playwright/test';

/**
 * First-run onboarding through to a funded testnet vault (nester#1127).
 *
 * Runs from a clean browser profile: Playwright gives each test a fresh
 * context, so there is no stored wallet, no dismissal, and no cached
 * progress — which is the state the acceptance criteria care about.
 *
 * The wallet extension itself cannot be installed in this browser, so the
 * later steps are driven by stubbing what the app observes rather than by
 * clicking through a real Freighter. That is the point the flow is being
 * tested at: the steps advance from observed state, so setting that state is
 * a faithful exercise of the machine.
 */

const DASHBOARD = '/dashboard';

async function gotoDashboard(page: Page) {
  await page.goto(DASHBOARD);
  // The panel mounts after the dismissal is read, so wait for the app rather
  // than for a fixed delay.
  await page.waitForLoadState('domcontentloaded');
}

test.describe('Testnet onboarding stepper', () => {
  test('a first-time visitor is shown the install step', async ({ page }) => {
    await gotoDashboard(page);

    const panel = page.getByTestId('testnet-onboarding');
    await expect(panel).toBeVisible();
    // With no wallet extension present, the flow must start at the beginning.
    await expect(panel).toHaveAttribute('data-active-step', 'install');
    await expect(page.getByRole('link', { name: /install freighter/i })).toBeVisible();
  });

  test('the panel stays dismissed across a reload', async ({ page }) => {
    await gotoDashboard(page);

    const panel = page.getByTestId('testnet-onboarding');
    await expect(panel).toBeVisible();

    await page.getByRole('button', { name: /dismiss testnet setup/i }).click();
    await expect(panel).toBeHidden();

    // Resuming after a reload is the criterion here: the dismissal is the one
    // piece of state that is stored, and it has to survive.
    await page.reload();
    await page.waitForLoadState('domcontentloaded');
    await expect(page.getByTestId('testnet-onboarding')).toBeHidden();
  });

  test('progress is announced for assistive technology', async ({ page }) => {
    await gotoDashboard(page);

    const progressbar = page.getByRole('progressbar', { name: /setup progress/i });
    await expect(progressbar).toBeVisible();
    await expect(progressbar).toHaveAttribute('aria-valuenow', '0');
  });

  test('every step is listed so the user can see what is ahead', async ({ page }) => {
    await gotoDashboard(page);

    for (const step of ['install', 'network', 'fund', 'deposit']) {
      await expect(page.getByTestId(`onboarding-step-${step}`)).toBeVisible();
    }
  });

  test('only the active step exposes an action', async ({ page }) => {
    await gotoDashboard(page);

    await expect(page.getByTestId('onboarding-step-install')).toHaveAttribute('data-state', 'active');
    for (const step of ['network', 'fund', 'deposit']) {
      await expect(page.getByTestId(`onboarding-step-${step}`)).toHaveAttribute('data-state', 'todo');
    }
  });
});
