// Date utilities for natural language parsing
// chrono-node: Import as namespace since ESM doesn't have default export
// esbuild will handle the bundling correctly
import * as chronoNs from 'chrono-node';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import customParseFormat from 'dayjs/plugin/customParseFormat';

// Access chrono-node: try default export first, then use namespace
// When bundled by esbuild, this should resolve correctly
const chrono = (chronoNs && chronoNs.default) ? chronoNs.default : chronoNs;

// Extend dayjs with plugins
dayjs.extend(relativeTime);
dayjs.extend(customParseFormat);

// Expose dayjs and chrono globally for potential use in other scripts
if (typeof window !== 'undefined') {
    window.dayjs = dayjs;
    window.chrono = chrono;
}

/**
 * Check if a date string represents a date-only (no specific time)
 * @param {string} isoString - ISO 8601 date string
 * @returns {boolean} - True if the date is at midnight (date-only)
 */
function isDateOnly(isoString) {
    if (!isoString) return false;
    const date = new Date(isoString);
    return date.getHours() === 0 && date.getMinutes() === 0 && date.getSeconds() === 0 && date.getMilliseconds() === 0;
}

/**
 * Parse a natural language date string into an ISO 8601 (RFC3339) string
 * Uses chrono-node for natural language parsing
 * @param {string} dateString - Natural language date string
 * @returns {string|null} - ISO 8601 string (RFC3339) or null if parsing fails
 */
export function parseNaturalDate(dateString) {
    if (!dateString || !dateString.trim()) {
        return null;
    }
    
    const input = dateString.trim();
    
    // Try parsing as ISO 8601 first
    const isoDate = new Date(input);
    if (!isNaN(isoDate.getTime()) && input.includes('T')) {
        return isoDate.toISOString();
    }
    
    // Use chrono-node for natural language parsing
    // Use forwardDate option to prefer future dates when ambiguous (e.g., "Friday" on Wednesday = this Friday, not last Friday)
    const referenceDate = new Date();
    const results = chrono.parse(input, referenceDate, { forwardDate: true });
    
    if (results.length > 0) {
        const firstResult = results[0];
        let parsedDate = firstResult.start.date();
        
        // Check if input explicitly contains time keywords
        // If not, normalize to midnight (date-only) - even if chrono-node set a time from reference date
        const hasExplicitTime = /\b(at|@|am|pm|morning|afternoon|evening|noon|midnight|\d{1,2}:\d{2})\b/i.test(input);
        
        // Also check if chrono-node detected time components in the parsed result
        // chrono-node might set time from reference date, so we need to check both
        const parsedHasTime = firstResult.start.get('hour') !== null || 
                               firstResult.start.get('minute') !== null ||
                               firstResult.start.get('second') !== null;
        
        // Only keep time if it was explicitly provided in the input OR detected by chrono-node
        // For relative dates like "in two weeks", normalize to midnight
        // Use both checks: explicit time keywords OR chrono-node detected time components
        if (!hasExplicitTime && !parsedHasTime) {
            parsedDate = new Date(parsedDate);
            parsedDate.setHours(0, 0, 0, 0);
        }
        
        return parsedDate.toISOString();
    }
    
    // Fallback: try JavaScript's Date constructor
    const fallbackDate = new Date(input);
    if (!isNaN(fallbackDate.getTime())) {
        return fallbackDate.toISOString();
    }
    
    return null;
}

/**
 * Format a date for display using dayjs
 * @param {string} isoString - ISO 8601 date string
 * @returns {string} - Formatted date string
 */
export function formatDate(isoString) {
    if (!isoString) return '';
    
    const date = dayjs(isoString);
    if (!date.isValid()) return '';
    
    const now = dayjs();
    const diffDays = date.diff(now, 'day');
    
    // Check if this is a date-only (no specific time)
    const dateOnly = isDateOnly(isoString);
    
    // Relative dates for near future
    if (diffDays === 0) {
        if (dateOnly) {
            return 'Today';
        }
        return `Today at ${date.format('h:mm A')}`;
    } else if (diffDays === 1) {
        if (dateOnly) {
            return 'Tomorrow';
        }
        return `Tomorrow at ${date.format('h:mm A')}`;
    } else if (diffDays > 1 && diffDays <= 7) {
        const dayName = date.format('dddd');
        if (dateOnly) {
            return dayName;
        }
        return `${dayName} at ${date.format('h:mm A')}`;
    } else {
        // Full date
        const dateStr = date.format('MMM D');
        const yearStr = date.year() !== now.year() ? `, ${date.year()}` : '';
        if (dateOnly) {
            return `${dateStr}${yearStr}`;
        }
        return `${dateStr}${yearStr} at ${date.format('h:mm A')}`;
    }
}

/**
 * Check if a date string is valid
 * @param {string} dateString - Date string to validate
 * @returns {boolean} - True if valid
 */
export function isValidDate(dateString) {
    if (!dateString || !dateString.trim()) {
        return false;
    }
    return parseNaturalDate(dateString) !== null;
}

/**
 * Extract date/time information from todo text and return cleaned text with detected date
 * Uses chrono-node for better natural language parsing
 * @param {string} text - Todo text that may contain date information
 * @returns {object} - { cleanedText: string, detectedDate: string|null, isDateOnly: boolean }
 */
export function extractDateFromText(text) {
    if (!text || !text.trim()) {
        return { cleanedText: text, detectedDate: null, isDateOnly: false };
    }
    
    const originalText = text.trim();
    const referenceDate = new Date();
    
    // Use chrono-node to parse dates from text
    // Parse with forwardDate option to prefer future dates when ambiguous
    const results = chrono.parse(originalText, referenceDate, { forwardDate: true });
    
    if (results.length === 0) {
        return { cleanedText: originalText, detectedDate: null, isDateOnly: false };
    }
    
    // Filter results to prefer future dates, then use the first (most confident) result
    // Compare dates at start of day (midnight) to avoid time-of-day issues
    const now = new Date();
    const nowStartOfDay = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const futureResults = results.filter(result => {
        const resultDate = result.start.date();
        const resultStartOfDay = new Date(resultDate.getFullYear(), resultDate.getMonth(), resultDate.getDate());
        // Prefer dates that are today or in the future
        return resultStartOfDay >= nowStartOfDay;
    });
    
    // Use future results if available, otherwise fall back to all results
    const resultToUse = futureResults.length > 0 ? futureResults[0] : results[0];
    let parsedDate = resultToUse.start.date();
    
    // Check if input explicitly contains time keywords
    // This is more reliable than checking parsed result, as chrono-node may copy reference time
    const hasExplicitTime = /\b(at|@|am|pm|morning|afternoon|evening|noon|midnight|\d{1,2}:\d{2})\b/i.test(originalText);
    
    // Also check if chrono-node detected time components in the parsed result
    const parsedHasTime = resultToUse.start.get('hour') !== null || 
                          resultToUse.start.get('minute') !== null ||
                          resultToUse.start.get('second') !== null;
    
    // Only keep time if it was explicitly provided in the input OR detected by chrono-node
    // For relative dates like "in two weeks", normalize to midnight
    // Use both checks: explicit time keywords OR chrono-node detected time components
    const dateOnly = !hasExplicitTime && !parsedHasTime;
    
    // If date-only, normalize to midnight
    if (dateOnly) {
        parsedDate = new Date(parsedDate);
        parsedDate.setHours(0, 0, 0, 0);
    }
    
    const isoString = parsedDate.toISOString();
    
    // Remove the matched text from the original
    let cleanedText = originalText;
    if (resultToUse.index !== undefined && resultToUse.text) {
        // Remove the matched text
        const before = originalText.substring(0, resultToUse.index);
        const after = originalText.substring(resultToUse.index + resultToUse.text.length);
        cleanedText = (before + ' ' + after).replace(/\s+/g, ' ').trim();
        // Remove leading/trailing punctuation
        cleanedText = cleanedText.replace(/^[,\s]+|[,\s]+$/g, '');
    }
    
    return { 
        cleanedText, 
        detectedDate: isoString, 
        isDateOnly: dateOnly 
    };
}

// Export isDateOnly for use in other modules
export { isDateOnly };

// Expose functions globally for backward compatibility with non-module scripts
if (typeof window !== 'undefined') {
    window.parseNaturalDate = parseNaturalDate;
    window.formatDate = formatDate;
    window.isValidDate = isValidDate;
    window.extractDateFromText = extractDateFromText;
    window.isDateOnly = isDateOnly;
}
