import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import enAbout from '@/locales/en/about.json'
import enCertificate from '@/locales/en/certificate.json'
import enCommon from '@/locales/en/common.json'
import enDecoder from '@/locales/en/decoder.json'
import enDirFinder from '@/locales/en/dir-finder.json'
import enEditorComponents from '@/locales/en/editor-components.json'
import enIntercept from '@/locales/en/intercept.json'
import enIntruder from '@/locales/en/intruder.json'
import enPassiveScan from '@/locales/en/passive-scan.json'
import enPortScanner from '@/locales/en/port-scanner.json'
import enProxyHistory from '@/locales/en/proxy-history.json'
import enRepeater from '@/locales/en/repeater.json'
import enRequestComponents from '@/locales/en/request-components.json'
import enSettings from '@/locales/en/settings.json'
import enSpider from '@/locales/en/spider.json'
import enSqliCheck from '@/locales/en/sqli-check.json'
import enSubdomainFinder from '@/locales/en/subdomain-finder.json'
import enWordpressCheck from '@/locales/en/wordpress-check.json'
import enXssCheck from '@/locales/en/xss-check.json'

import ptAbout from '@/locales/pt/about.json'
import ptCertificate from '@/locales/pt/certificate.json'
import ptCommon from '@/locales/pt/common.json'
import ptDecoder from '@/locales/pt/decoder.json'
import ptDirFinder from '@/locales/pt/dir-finder.json'
import ptEditorComponents from '@/locales/pt/editor-components.json'
import ptIntercept from '@/locales/pt/intercept.json'
import ptIntruder from '@/locales/pt/intruder.json'
import ptPassiveScan from '@/locales/pt/passive-scan.json'
import ptPortScanner from '@/locales/pt/port-scanner.json'
import ptProxyHistory from '@/locales/pt/proxy-history.json'
import ptRepeater from '@/locales/pt/repeater.json'
import ptRequestComponents from '@/locales/pt/request-components.json'
import ptSettings from '@/locales/pt/settings.json'
import ptSpider from '@/locales/pt/spider.json'
import ptSqliCheck from '@/locales/pt/sqli-check.json'
import ptSubdomainFinder from '@/locales/pt/subdomain-finder.json'
import ptWordpressCheck from '@/locales/pt/wordpress-check.json'
import ptXssCheck from '@/locales/pt/xss-check.json'

export const STORAGE_KEY = 'nullsec-lang'
export type Lang = 'en' | 'pt'

function initialLang(): Lang {
  if (typeof window === 'undefined') return 'en'
  const stored = window.localStorage.getItem(STORAGE_KEY)
  return stored === 'pt' ? 'pt' : 'en'
}

// Every resource below is a static import already in memory (no backend
// plugin fetching anything over the network), so init() resolves in the
// same tick — tests and the first render never race a still-loading
// instance.
i18n.use(initReactI18next).init({
  resources: {
    en: {
      common: enCommon,
      about: enAbout,
      settings: enSettings,
      'dir-finder': enDirFinder,
      'xss-check': enXssCheck,
      'sqli-check': enSqliCheck,
      intruder: enIntruder,
      repeater: enRepeater,
      intercept: enIntercept,
      'proxy-history': enProxyHistory,
      'subdomain-finder': enSubdomainFinder,
      'port-scanner': enPortScanner,
      spider: enSpider,
      'passive-scan': enPassiveScan,
      'wordpress-check': enWordpressCheck,
      certificate: enCertificate,
      decoder: enDecoder,
      'editor-components': enEditorComponents,
      'request-components': enRequestComponents,
    },
    pt: {
      common: ptCommon,
      about: ptAbout,
      settings: ptSettings,
      'dir-finder': ptDirFinder,
      'xss-check': ptXssCheck,
      'sqli-check': ptSqliCheck,
      intruder: ptIntruder,
      repeater: ptRepeater,
      intercept: ptIntercept,
      'proxy-history': ptProxyHistory,
      'subdomain-finder': ptSubdomainFinder,
      'port-scanner': ptPortScanner,
      spider: ptSpider,
      'passive-scan': ptPassiveScan,
      'wordpress-check': ptWordpressCheck,
      certificate: ptCertificate,
      decoder: ptDecoder,
      'editor-components': ptEditorComponents,
      'request-components': ptRequestComponents,
    },
  },
  lng: initialLang(),
  fallbackLng: 'en',
  defaultNS: 'common',
  interpolation: { escapeValue: false },
  react: { useSuspense: false },
})

export function setLang(lang: Lang) {
  window.localStorage.setItem(STORAGE_KEY, lang)
  i18n.changeLanguage(lang)
}

export default i18n
