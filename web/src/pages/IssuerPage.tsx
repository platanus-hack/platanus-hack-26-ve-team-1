import { useState } from 'react'
import { CardsPanel } from '../components/issuer/CardsPanel'
import { UniversityPanel } from '../components/issuer/UniversityPanel'
import { PageTitle } from '../components/PageTitle'
import { Tabs, type TabItem } from '../components/ui'

const TABS: TabItem[] = [
  { id: 'cedulas', label: 'Cédulas', panel: <CardsPanel /> },
  { id: 'universidad', label: 'Universidad', panel: <UniversityPanel /> },
]

export default function IssuerPage() {
  const [activeTab, setActiveTab] = useState(TABS[0].id)

  return (
    <div className="flex flex-col gap-6">
      <PageTitle>Panel del emisor</PageTitle>
      <Tabs
        label="Secciones del panel del emisor"
        items={TABS}
        activeId={activeTab}
        onChange={setActiveTab}
      />
    </div>
  )
}
