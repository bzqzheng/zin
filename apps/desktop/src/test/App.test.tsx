import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import App from '../App'

describe('App', () => {
  it('renders the three-panel layout', () => {
    render(<App />)

    expect(screen.getByText('Zin')).toBeVisible()
    expect(screen.getByText('Agent Workspace')).toBeVisible()
    expect(screen.getByText('Projects')).toBeVisible()
    expect(screen.getByText('Issues')).toBeVisible()
    expect(screen.getByText('Tauri Frontend Scaffold')).toBeVisible()
    expect(screen.getByText('Timeline')).toBeVisible()
    expect(screen.getAllByText('Trinity').length).toBeGreaterThanOrEqual(2)
  })

  it('renders the sidebar with issue list', () => {
    render(<App />)
    expect(screen.getAllByText(/ZIN-\d/).length).toBeGreaterThan(0)
  })

  it('renders timeline events', () => {
    render(<App />)
    expect(screen.getByText('Issue created')).toBeVisible()
    expect(screen.getByText('Assigned to Trinity')).toBeVisible()
    expect(screen.getByText('Status: In Progress')).toBeVisible()
  })
})
