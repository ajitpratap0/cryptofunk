import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'

// Mock next/navigation
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => '/',
}))

import { Sidebar } from '@/components/layout/Sidebar'

describe('Sidebar', () => {
  it('renders navigation links when expanded', () => {
    render(<Sidebar collapsed={false} onToggle={vi.fn()} />)
    
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
    expect(screen.getByText('Live Trades')).toBeInTheDocument()
    expect(screen.getByText('Performance')).toBeInTheDocument()
    expect(screen.getByText('Agents')).toBeInTheDocument()
    expect(screen.getByText('Risk')).toBeInTheDocument()
    expect(screen.getByText('Settings')).toBeInTheDocument()
  })

  it('renders collapsed state without text labels', () => {
    render(<Sidebar collapsed={true} onToggle={vi.fn()} />)
    
    // In collapsed mode, text labels are hidden
    expect(screen.queryByText('Dashboard')).not.toBeInTheDocument()
  })

  it('renders the CryptoFunk branding when expanded', () => {
    render(<Sidebar collapsed={false} onToggle={vi.fn()} />)
    expect(screen.getByText('CryptoFunk')).toBeInTheDocument()
  })
})
