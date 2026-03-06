import { BrowserRouter, Routes, Route } from "react-router"
import Dashboard from "@/pages/Dashboard"
import PipelineDetail from "@/pages/PipelineDetail"

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/pipeline/:id" element={<PipelineDetail />} />
      </Routes>
    </BrowserRouter>
  )
}

export default App
