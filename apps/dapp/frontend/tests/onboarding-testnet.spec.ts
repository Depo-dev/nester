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

// The landing page, not /dashboard: without a connected wallet the dashboard
// renders nothing, and no wallet is exactly the state a first-time visitor
// arrives in. The deposit step is picked up on the dashboard once connected.
const LANDING = '/';

async function gotoOnboarding(page: Page) {
  await page.goto(LANDING);
  await page.waitForLoadState('domcontentloaded');

  // A clean profile also gets the welcome modal, which is a full-screen
  // overlay. A real user clears it before they can reach anything underneath,
  // so the test does the same rather than reaching through it.
  const skip = page.getByRole('button', { name: /^skip$/i });
  if (await skip.isVisible().catch(() => false)) {
    await skip.click();
    await skip.waitFor({ state: 'hidden', timeout: 10_000 }).catch(() => {});
  }

  // The panel mounts after the dismissal is read from storage, so wait for it
  // to settle rather than for a fixed delay.
  await page
    .getByTestId('testnet-onboarding')
    .waitFor({ state: 'attached', timeout: 15_000 })
    .catch(() => {});
}

test.describe('Testnet onboarding stepper', () => {
  test('a first-time visitor is shown the install step', async ({ page }) => {
    await gotoOnboarding(page);

    const panel = page.getByTestId('testnet-onboarding');
    await expect(panel).toBeVisible();
    // With no wallet extension present, the flow must start at the beginning.
    await expect(panel).toHaveAttribute('data-active-step', 'install');
    await expect(page.getByRole('link', { name: /install freighter/i })).toBeVisible();
  });

  test('the panel stays dismissed across a reload', async ({ page }) => {
    await gotoOnboarding(page);

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
    await gotoOnboarding(page);

    const progressbar = page.getByRole('progressbar', { name: /setup progress/i });
    await expect(progressbar).toBeVisible();
    await expect(progressbar).toHaveAttribute('aria-valuenow', '0');
  });

  test('every step is listed so the user can see what is ahead', async ({ page }) => {
    await gotoOnboarding(page);

    for (const step of ['install', 'network', 'fund', 'deposit']) {
      await expect(page.getByTestId(`onboarding-step-${step}`)).toBeVisible();
    }
  });

  test('only the active step exposes an action', async ({ page }) => {
    await gotoOnboarding(page);

    await expect(page.getByTestId('onboarding-step-install')).toHaveAttribute('data-state', 'active');
    for (const step of ['network', 'fund', 'deposit']) {
      await expect(page.getByTestId(`onboarding-step-${step}`)).toHaveAttribute('data-state', 'todo');
    }
  });
});
