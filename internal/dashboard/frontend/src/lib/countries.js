const ALPHA3_TO_ALPHA2 = {
  AFG:'AF',ALB:'AL',DZA:'DZ',ASM:'AS',AND:'AD',AGO:'AO',AIA:'AI',ATA:'AQ',ATG:'AG',ARG:'AR',
  ARM:'AM',ABW:'AW',AUS:'AU',AUT:'AT',AZE:'AZ',BHS:'BS',BHR:'BH',BGD:'BD',BRB:'BB',BLR:'BY',
  BEL:'BE',BLZ:'BZ',BEN:'BJ',BMU:'BM',BTN:'BT',BOL:'BO',BIH:'BA',BWA:'BW',BRA:'BR',BRN:'BN',
  BGR:'BG',BFA:'BF',BDI:'BI',KHM:'KH',CMR:'CM',CAN:'CA',CPV:'CV',CYM:'KY',CAF:'CF',TCD:'TD',
  CHL:'CL',CHN:'CN',COL:'CO',COM:'KM',COG:'CG',COD:'CD',COK:'CK',CRI:'CR',CIV:'CI',HRV:'HR',
  CUB:'CU',CYP:'CY',CZE:'CZ',DNK:'DK',DJI:'DJ',DMA:'DM',DOM:'DO',ECU:'EC',EGY:'EG',SLV:'SV',
  GNQ:'GQ',ERI:'ER',EST:'EE',ETH:'ET',FLK:'FK',FRO:'FO',FJI:'FJ',FIN:'FI',FRA:'FR',GUF:'GF',
  PYF:'PF',GAB:'GA',GMB:'GM',GEO:'GE',DEU:'DE',GHA:'GH',GIB:'GI',GRC:'GR',GRL:'GL',GRD:'GD',
  GLP:'GP',GUM:'GU',GTM:'GT',GIN:'GN',GNB:'GW',GUY:'GY',HTI:'HT',HND:'HN',HKG:'HK',HUN:'HU',
  ISL:'IS',IND:'IN',IDN:'ID',IRN:'IR',IRQ:'IQ',IRL:'IE',ISR:'IL',ITA:'IT',JAM:'JM',JPN:'JP',
  JOR:'JO',KAZ:'KZ',KEN:'KE',KIR:'KI',PRK:'KP',KOR:'KR',KWT:'KW',KGZ:'KG',LAO:'LA',LVA:'LV',
  LBN:'LB',LSO:'LS',LBR:'LR',LBY:'LY',LIE:'LI',LTU:'LT',LUX:'LU',MAC:'MO',MKD:'MK',MDG:'MG',
  MWI:'MW',MYS:'MY',MDV:'MV',MLI:'ML',MLT:'MT',MHL:'MH',MTQ:'MQ',MRT:'MR',MUS:'MU',MEX:'MX',
  FSM:'FM',MDA:'MD',MCO:'MC',MNG:'MN',MNE:'ME',MSR:'MS',MAR:'MA',MOZ:'MZ',MMR:'MM',NAM:'NA',
  NRU:'NR',NPL:'NP',NLD:'NL',NCL:'NC',NZL:'NZ',NIC:'NI',NER:'NE',NGA:'NG',NIU:'NU',NFK:'NF',
  MNP:'MP',NOR:'NO',OMN:'OM',PAK:'PK',PLW:'PW',PSE:'PS',PAN:'PA',PNG:'PG',PRY:'PY',PER:'PE',
  PHL:'PH',PCN:'PN',POL:'PL',PRT:'PT',PRI:'PR',QAT:'QA',REU:'RE',ROU:'RO',RUS:'RU',RWA:'RW',
  SHN:'SH',KNA:'KN',LCA:'LC',SPM:'PM',VCT:'VC',WSM:'WS',SMR:'SM',STP:'ST',SAU:'SA',SEN:'SN',
  SRB:'RS',SYC:'SC',SLE:'SL',SGP:'SG',SVK:'SK',SVN:'SI',SLB:'SB',SOM:'SO',ZAF:'ZA',ESP:'ES',
  LKA:'LK',SDN:'SD',SUR:'SR',SWZ:'SZ',SWE:'SE',CHE:'CH',SYR:'SY',TWN:'TW',TJK:'TJ',TZA:'TZ',
  THA:'TH',TLS:'TL',TGO:'TG',TKL:'TK',TON:'TO',TTO:'TT',TUN:'TN',TUR:'TR',TKM:'TM',TCA:'TC',
  TUV:'TV',UGA:'UG',UKR:'UA',ARE:'AE',GBR:'GB',USA:'US',URY:'UY',UZB:'UZ',VUT:'VU',VEN:'VE',
  VNM:'VN',VGB:'VG',VIR:'VI',WLF:'WF',YEM:'YE',ZMB:'ZM',ZWE:'ZW',SSD:'SS',XKX:'XK',CUW:'CW',
  SXM:'SX',BES:'BQ',
};

export const countryCodeToAlpha2 = (code) => {
  if (!code) return null;
  const upper = String(code).trim().toUpperCase();
  const a2 = upper.length === 2 ? upper : ALPHA3_TO_ALPHA2[upper];
  return /^[A-Z]{2}$/.test(a2 || '') ? a2 : null;
};

export const countryCodeToFlag = (code) => {
  const a2 = countryCodeToAlpha2(code);
  if (!a2) return null;
  const codePoints = [...a2].map((c) => 0x1F1E6 + c.charCodeAt(0) - 65);
  return String.fromCodePoint(...codePoints);
};

// Intl.DisplayNames carries every region name the browser already ships for
// its own locale data, so the names cost us no bundle weight and stay correct
// as countries rename themselves. The names are pinned to English because the
// rest of the dashboard is.
let regionNames;
let regionNamesResolved = false;
const englishRegionNames = () => {
  if (!regionNamesResolved) {
    regionNamesResolved = true;
    try {
      regionNames = new Intl.DisplayNames(['en'], { type: 'region' });
    } catch {
      regionNames = null;
    }
  }
  return regionNames;
};

// Falls back to the bare code whenever the platform cannot name the region, so
// a tooltip never reads worse than the code it replaced.
export const countryCodeToName = (code) => {
  const a2 = countryCodeToAlpha2(code);
  if (!a2) return null;
  const names = englishRegionNames();
  if (!names) return a2;
  let name;
  try {
    name = names.of(a2);
  } catch {
    return a2;
  }
  if (!name || name === a2 || name === 'Unknown Region') return a2;
  return name;
};
