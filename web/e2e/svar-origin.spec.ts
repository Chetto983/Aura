import { expect, test } from '@playwright/test';
import { gotoAuthenticated } from './auth';
import { expectNothingLeftTheAppliance, watchNetwork } from './support/network';

// The SVAR widgets' CSS declares Open Sans and Roboto against cdn.svar.dev; src/styles/svar.css
// redeclares them against this origin, and only wins when it comes after every package sheet.
// When the board started importing that file too, the bundler moved it into a chunk loaded
// first, and a surface that embeds the file manager fetched Roboto from the CDN. This walks the
// surfaces that load SVAR CSS, the board and the documents (the file manager and the grid), with
// no store behind them, and holds every request to the appliance's own origin (measured: the
// out-of-order build fails it with cdn.svar.dev/fonts/roboto/regular.woff2, 2026-10-09).
test('the surfaces that load SVAR widgets fetch nothing from another origin', async ({
  page,
}, info) => {
  const network = watchNetwork(page);
  await gotoAuthenticated(page, '/');
  const nav =
    info.project.name === 'chrome'
      ? page.getByRole('navigation', { name: /Primary|Principale/ })
      : page.getByRole('navigation', { name: /Modes|Modalit/ });
  await nav.getByRole('button', { name: /^(Board|Bacheca)$/ }).click();
  await expect(page.getByRole('region', { name: 'Kanban board' })).toBeVisible();
  await nav.getByRole('button', { name: /^(Documents|Documenti|Docs|Doc)$/ }).click();
  await expect(page.getByRole('region', { name: /Documents|Documenti/ })).toBeVisible();
  // Fonts load when text first needs them, after layout; give the browser that turn.
  await page.waitForTimeout(1500);
  await expectNothingLeftTheAppliance(page, network);
});
