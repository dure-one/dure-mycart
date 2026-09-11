import { Page } from 'patchright'
import { expect } from 'patchright/test'

/**
 * Feature Object for Admin Product Variants Management
 * Handles product creation with variants (options like Size, Color)
 */
export class AdminProductVariantsFeature {
  constructor(private page: Page) {}

  async login() {
    if (this.page.url().includes('/_/signin')) {
      await this.page.locator('input[name="email"], input[type="email"]').fill('admin@example.com')
      await this.page.locator('input[name="password"], input[type="password"]').fill('test1234')
      await this.page.locator('button[type="submit"]').click()
      await this.page.waitForURL('/_/**', { timeout: 10000 })
    }
  }

  async goto() {
    await this.page.goto('/_/products')
    await this.page.waitForLoadState('networkidle')
    await this.login()
  }

  async waitForPage() {
    await this.page.waitForSelector('h1', { timeout: 10000 })
    await this.page.waitForTimeout(500)
  }

  async getProductCount() {
    return await this.page.locator('[data-testid="product-row"]').count()
  }

  async openAddProductDrawer() {
    await this.waitForPage()
    const addButton = this.page.locator('button').filter({ hasText: /add product/i })
    await addButton.first().waitFor({ state: 'visible', timeout: 10000 })
    await addButton.first().click()
    await this.page.getByRole('textbox', { name: 'Name' }).waitFor({ state: 'visible', timeout: 10000 })
  }

  async fillBasicProductInfo(data: { name: string; slug: string; brief: string; amount: string; digitalType?: 'file' | 'data' | 'api' }) {
    await this.page.getByRole('textbox', { name: 'Name' }).fill(data.name)
    await this.page.getByRole('textbox', { name: 'URL' }).fill(data.slug)
    await this.page.getByRole('textbox', { name: 'Brief' }).fill(data.brief)
    await this.page.getByRole('textbox', { name: 'Amount' }).fill(data.amount)

    // Select digital type if provided (required for new products)
    if (data.digitalType) {
      await this.page.getByRole('combobox', { name: 'Digital type' }).selectOption(data.digitalType)
    }
  }

  async toggleProductVariants() {
    // Find the Product Variants heading
    const variantHeading = this.page.locator('h3', { hasText: /product variants/i })
    await variantHeading.waitFor({ state: 'visible', timeout: 5000 })

    // Find the toggle checkbox - it's in the parent's sibling container
    // Navigate from h3 -> parent div -> parent flex container -> label with checkbox
    const toggleLabel = variantHeading.locator('../..').locator('label[for^="toggle_"]')
    await toggleLabel.waitFor({ state: 'visible', timeout: 5000 })

    // Click the label (which toggles the checkbox)
    await toggleLabel.click()
    await this.page.waitForTimeout(500)
  }

  async addOption(optionName: string, values: string[]) {
    // Check if this is the first option (auto-created on toggle) or need to add new option
    const existingOptions = this.page.locator('input[placeholder*="Size, Color, Material" i]')
    const optionCount = await existingOptions.count()

    let isUsingExistingOption = false

    if (optionCount === 0) {
      // No existing options, click "Add Option" button
      const addOptionButton = this.page.locator('button', { hasText: /add option/i })
      await addOptionButton.first().click()
    } else {
      // Check if first option is empty (auto-created)
      const firstOptionValue = await existingOptions.first().inputValue()
      if (firstOptionValue.trim() === '') {
        // First option is empty, use it instead of creating new one
        isUsingExistingOption = true
        await existingOptions.first().fill(optionName)
      } else {
        // First option already filled, add new option
        const addOptionButton = this.page.locator('button', { hasText: /add option/i })
        await addOptionButton.first().click()

        // Wait for the new option editor to appear
        const newOptionInput = this.page.locator('input[placeholder*="Size, Color, Material" i]').last()
        await newOptionInput.waitFor({ state: 'visible', timeout: 5000 })
        await newOptionInput.fill(optionName)
      }
    }

    // Fill values for this option
    // Wait for at least one value input to appear (each option comes with one empty value input by default)
    const valueInputsLocator = this.page.locator('input[placeholder*="Value" i]')
    await valueInputsLocator.last().waitFor({ state: 'visible', timeout: 5000 })

    // Small delay to ensure DOM is stable
    await this.page.waitForTimeout(200)

    // Get current value inputs
    const valueInputs = this.page.locator('input[placeholder*="Value" i]')

    // Track how many values we've filled
    let valuesFilled = 0

    // Check the LAST value input (should be the empty one from the current option)
    const lastValueInput = valueInputs.last()
    const lastValue = await lastValueInput.inputValue()

    if (lastValue.trim() === '') {
      // Fill the existing empty value input with the first value
      await lastValueInput.fill(values[0])
      valuesFilled = 1
    }

    // Add remaining values
    for (let i = valuesFilled; i < values.length; i++) {
      const value = values[i]

      // Click "Add Value" button for this option
      const addValueButtons = this.page.locator('button', { hasText: /add value/i })
      const buttonCount = await addValueButtons.count()
      await addValueButtons.nth(buttonCount - 1).click()

      // Wait for the new value input to appear and fill it
      const updatedValueInputs = this.page.locator('input[placeholder*="Value" i]')
      await updatedValueInputs.last().waitFor({ state: 'visible', timeout: 3000 })
      await updatedValueInputs.last().fill(value)
    }
  }

  async fillVariantDetails(variants: Array<{ sku: string; quantity: number; priceSurcharge?: number }>) {
    await this.page.waitForTimeout(1000)

    // Fill variant details using ID-based locators for reliability
    for (let i = 0; i < variants.length; i++) {
      const variant = variants[i]

      // Fill SKU
      const skuInput = this.page.locator(`#variant-${i}-sku`)
      await skuInput.waitFor({ state: 'visible', timeout: 5000 })
      await skuInput.fill(variant.sku)

      // Fill Price Surcharge (defaults to 0 if not provided)
      const priceInput = this.page.locator(`#variant-${i}-price`)
      await priceInput.waitFor({ state: 'visible', timeout: 5000 })
      await priceInput.fill(String(variant.priceSurcharge ?? 0))

      // Fill Quantity
      const qtyInput = this.page.locator(`#variant-${i}-quantity`)
      await qtyInput.waitFor({ state: 'visible', timeout: 5000 })
      await qtyInput.fill(String(variant.quantity))
    }

    await this.page.waitForTimeout(500)
  }

  async saveProduct() {
    // Use proven selector pattern with multiple fallbacks (matches admin-products.feature.ts)
    const saveButton = this.page.locator('button:has-text("SAVE"), button:has-text("Save"), button[type="submit"]')
    await saveButton.first().click()

    // Wait for drawer to close (Name field should disappear after successful save)
    await this.page.getByRole('textbox', { name: 'Name' }).waitFor({ state: 'hidden', timeout: 10000 })
  }

  async verifyProductExists(productName: string) {
    const product = this.page.getByText(productName, { exact: false })
    await expect(product).toBeVisible({ timeout: 10000 })
  }

  async openProductForEdit(productName: string) {
    // Find and click the product row
    const productRow = this.page.locator('[data-testid="product-row"]').filter({ hasText: productName })
    await productRow.first().click()
    await this.page.waitForTimeout(1000)
  }

  async verifyVariantsExist() {
    // Check if Product Variants section is visible
    const variantsSection = this.page.locator('text=/product variants|options/i')
    await expect(variantsSection.first()).toBeVisible({ timeout: 5000 })
  }

  async verifyOptionExists(optionName: string) {
    const option = this.page.getByText(optionName, { exact: false })
    await expect(option.first()).toBeVisible({ timeout: 5000 })
  }
}
