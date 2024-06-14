import { test, expect, Page } from "@playwright/test";

test("happy path", async ({ page, browser }) => {
    test.setTimeout(60_000);
    await page.goto(process.env.BASE_URL || "localhost:8080");
    await expect(page).toHaveURL(/first-time/);

    const nameInput = page.getByPlaceholder("Enter username...");
    await nameInput.fill("Test user");
    await nameInput.press("Enter");

    await page.getByRole("button", { name: "Create" }).click();
    await expect(page).toHaveURL(/room\/.+/);
    const roomUrl = page.url();

    const queries = Array.from({ length: 5 }, (_, i) => String.fromCodePoint(i + "a".charCodeAt(0)));
    const n = queries.length + 1
    const pages = await Promise.all(
        queries.map(async () => {
            const ctx = await browser.newContext();
            return await ctx.newPage();
        })
    );

    pages.forEach((p, i) => {
        simulateUserAddingSuggestions(p, roomUrl, queries[i]);
    });

    await page.getByRole("button", { name: "Ready" }).click();
    await expect(page.locator("#players summary")).toHaveText(`${n}/${n}`);

    await page.getByRole("button", { name: "Next" }).click();

    await Promise.all(
        pages.map(async (p) => {
            return await expect(p.locator("#players summary")).toHaveText(
                `0/${n}`
            );
        })
    );

    await Promise.all(
        pages.map(async (page) => {
            const size = page.viewportSize()!;
            const from = { x: size.width / 2, y: size.height / 2 };
            const to = { x: from.x + 200, y: from.y };

            let count = 0;
            while (
                (await page.locator("#remains_total").textContent()) !=
                "Remains: 0" &&
                count++ < 500
            ) {
                await swipe(page, from, to);
            }

            await expect(page.locator("#remains_total")).toHaveText(
                "Remains: 0"
            );
        })
    );

    await expect(page.locator("#players summary")).toHaveText(`${n - 1}/${n}`);
    await page.getByRole("button", { name: "Next" }).click();

    await Promise.all(
        pages.map(async (p) => {
            await p.getByRole("link", { name: "Leave" }).click();
            await expect(p).toHaveURL(/first-time/);
        })
    );

    await page.getByRole("link", { name: "Leave" }).click();
    await expect(page).toHaveURL(new RegExp(process.env.BASE_URL || "localhost:8080" + "/$"));
});

async function simulateUserAddingSuggestions(
    page: Page,
    url: string,
    query: string
) {
    await page.goto(url);
    const search = page.getByPlaceholder(/search movies/i);

    await search.focus();
    await search.fill(query);

    const ul = page.locator("#search_results");
    await expect(ul).toHaveText(new RegExp(query, "i"));

    const nOfSuggestions = await ul.locator("> li").count();

    for (let i = 0; i < nOfSuggestions; i++) {
        await search.press("ArrowDown");
        await search.press("Enter");
    }

    await search.press("Escape");
    await page.getByRole("button", { name: "Ready" }).click();
}

async function swipe(
    page: Page,
    from: { x: number; y: number },
    to: { x: number; y: number }
) {
    page.mouse.move(from.x, from.y);
    page.mouse.down();
    page.mouse.move(to.x, to.y);
    page.mouse.up();
}
