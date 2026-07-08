"""Take screenshots of all LabOps pages."""
import os, sys
sys.path.insert(0, r'C:\Users\cowho\AppData\Local\Programs\Python\Python312\Lib\site-packages')

from playwright.sync_api import sync_playwright

os.makedirs('docs/screenshots', exist_ok=True)

with sync_playwright() as p:
    b = p.chromium.launch(headless=True)
    page = b.new_page(viewport={'width': 1440, 'height': 900})

    pages = [
        ('01-login', '/login'),
        ('02-dashboard', '/dashboard'),
        ('03-devices', '/devices'),
        ('04-groups', '/groups'),
        ('05-tasks', '/tasks'),
        ('06-audit', '/audit'),
        ('07-aiops', '/aiops'),
    ]

    # Login first
    page.goto('http://localhost:5173/login')
    page.wait_for_load_state('networkidle')
    page.screenshot(path='docs/screenshots/01-login.png', full_page=True)
    print('01-login done')

    page.fill('input[id="username"]', 'admin')
    page.fill('input[id="password"]', 'admin')
    page.click('button[type="submit"]')
    page.wait_for_url('**/dashboard', timeout=10000)
    page.wait_for_load_state('networkidle')
    page.wait_for_timeout(2000)
    page.screenshot(path='docs/screenshots/02-dashboard.png', full_page=True)
    print('02-dashboard done')

    # Remaining pages
    for name, path in pages[2:]:
        page.goto(f'http://localhost:5173{path}')
        page.wait_for_load_state('networkidle')
        page.wait_for_timeout(800)
        page.screenshot(path=f'docs/screenshots/{name}.png', full_page=True)
        print(f'{name} done')

    b.close()
    print('All 7 screenshots saved.')
