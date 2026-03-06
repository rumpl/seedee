import { BrowserRouter, Routes, Route } from "react-router"
import Dashboard from "@/pages/Dashboard"
import PipelineDetail from "@/pages/PipelineDetail"
import NotFound from "@/pages/NotFound"
import { Toaster } from "@/components/Toaster"
import { ConnectionBanner } from "@/components/ConnectionBanner"

function App() {
  return (
    <BrowserRouter>
      <ConnectionBanner />
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/pipeline/:id" element={<PipelineDetail />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
      <Toaster />
    </BrowserRouter>
  )
}

export default App
