import { useState } from 'react'
import { AlertCircle, CheckCircle2, Loader2, Search, UserRoundSearch } from 'lucide-react'
import { reviewApi } from '../api/reviewApi.js'

export function CustomerProfilePicker({ token, selected, onSelect }) {
  const [query, setQuery] = useState('')
  const [customers, setCustomers] = useState([])
  const [searching, setSearching] = useState(false)
  const [searchError, setSearchError] = useState('')
  const [searched, setSearched] = useState(false)

  async function handleSearch(event) {
    event.preventDefault()
    const trimmed = query.trim()
    if (trimmed.length < 3) {
      setSearchError('Minimal 3 karakter untuk mencari profil.')
      setCustomers([])
      setSearched(false)
      return
    }
    setSearching(true)
    setSearchError('')
    onSelect(null)
    try {
      const response = await reviewApi.searchCustomers(token, trimmed)
      setCustomers(response?.data || [])
      setSearched(true)
    } catch (error) {
      setSearchError(error.message || 'Pencarian profil gagal.')
      setCustomers([])
      setSearched(false)
    } finally {
      setSearching(false)
    }
  }

  return (
    <section className="panel review-panel">
      <div className="panel-heading">
        <h2>Pilih profil pelanggan</h2>
        <span className="count-chip">{selected ? '1 dipilih' : 'belum dipilih'}</span>
      </div>
      <form onSubmit={handleSearch} className="search-form">
        <label className="field">
          <span>Pencarian profil (nama/HP)</span>
          <div className="search-input">
            <Search size={15} />
            <input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Minimal 3 karakter" />
            <button type="submit" className="btn btn-primary btn-small" disabled={searching}>
              {searching ? <Loader2 size={14} className="spin" /> : 'Cari'}
            </button>
          </div>
        </label>
      </form>

      {searchError ? (
        <div className="alert alert-error" role="alert"><AlertCircle size={15} /><span>{searchError}</span></div>
      ) : null}

      {customers.length > 0 ? (
        <ul className="candidate-list">
          {customers.map((customer) => (
            <li key={customer.id}>
              <button
                type="button"
                className={`candidate-row ${selected?.id === customer.id ? 'is-selected' : ''}`}
                onClick={() => onSelect(customer)}
              >
                <span className="candidate-icon"><UserRoundSearch size={16} /></span>
                <span className="candidate-body">
                  <strong>{customer.name}</strong>
                  <small>{customer.phone}{customer.email ? ` · ${customer.email}` : ''}</small>
                  {customer.address ? <small className="candidate-address">{customer.address}</small> : null}
                </span>
              </button>
            </li>
          ))}
        </ul>
      ) : null}

      {searched && customers.length === 0 && !searching ? <p className="empty-hint">Tidak ada profil yang cocok. Coba kata kunci lain.</p> : null}

      {selected ? (
        <div className="selected-customer">
          <CheckCircle2 size={16} />
          <span>Profil terpilih: <strong>{selected.name}</strong> ({selected.phone})</span>
        </div>
      ) : null}
    </section>
  )
}
