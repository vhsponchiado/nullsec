import { useTranslation } from 'react-i18next'
import { setLang, type Lang } from '@/lib/i18n'
import { cn } from '@/lib/utils'

const LANGS: { code: Lang; label: string }[] = [
  { code: 'en', label: 'EN' },
  { code: 'pt', label: 'PT' },
]

export function LanguageSwitcher() {
  const { i18n } = useTranslation()
  const current = i18n.language === 'pt' ? 'pt' : 'en'

  return (
    <div className="flex items-center rounded-md border p-0.5 text-xs font-mono">
      {LANGS.map((l) => (
        <button
          key={l.code}
          onClick={() => setLang(l.code)}
          aria-pressed={current === l.code}
          className={cn(
            'px-2 py-1 rounded-sm cursor-pointer transition-colors',
            current === l.code
              ? 'bg-primary text-primary-foreground'
              : 'text-muted-foreground hover:text-foreground',
          )}
        >
          {l.label}
        </button>
      ))}
    </div>
  )
}
