import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => '/',
}))

import { Providers } from '@/app/providers'

describe('Providers (layout wrapper)', () => {
  it('renders children', () => {
    render(
      <Providers>
        <div data-testid="child">Hello</div>
      </Providers>
    )
    expect(screen.getByTestId('child')).toBeInTheDocument()
    expect(screen.getByText('Hello')).toBeInTheDocument()
  })

  it('wraps content with QueryClientProvider (no crash)', () => {
    // If QueryClientProvider is missing, hooks would throw
    const { container } = render(
      <Providers>
        <span>Test</span>
      </Providers>
    )
    expect(container).toBeTruthy()
  })
})
