import { Routes, Route } from 'react-router-dom'
import { Layout } from '../components/layout/Layout'
import { AlertWorkbench } from '../pages/AlertWorkbench'
import { RCAReport } from '../pages/RCAReport'
import { History } from '../pages/History'
import { Settings } from '../pages/Settings'

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<AlertWorkbench />} />
        <Route path="/reports/:id" element={<RCAReport />} />
        <Route path="/history" element={<History />} />
        <Route path="/settings" element={<Settings />} />
      </Route>
    </Routes>
  )
}
