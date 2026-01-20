// Date utilities for natural language parsing

/**
 * Parse a natural language date string into an ISO 8601 (RFC3339) string
 * Supports formats like:
 * - "tomorrow", "tomorrow at 3pm", "tomorrow at 3:30pm"
 * - "next friday", "next friday at 2pm"
 * - "in 3 days", "in 2 weeks"
 * - "2024-03-15", "March 15, 2024", "03/15/2024"
 * - "today at 5pm", "today"
 * - Standard ISO 8601 formats
 * @param {string} dateString - Natural language date string
 * @returns {string|null} - ISO 8601 string (RFC3339) or null if parsing fails
 */
function parseNaturalDate(dateString) {
    if (!dateString || !dateString.trim()) {
        return null;
    }
    
    const input = dateString.trim().toLowerCase();
    const now = new Date();
    let date = new Date();
    
    // Try parsing as ISO 8601 first
    const isoDate = new Date(dateString);
    if (!isNaN(isoDate.getTime()) && dateString.includes('T')) {
        return isoDate.toISOString();
    }
    
    // Handle "tomorrow" and variations
    if (input.startsWith('tomorrow')) {
        date.setDate(now.getDate() + 1);
        date.setHours(0, 0, 0, 0);
        
        // Check for time specification
        const timeMatch = input.match(/(\d{1,2})(?::(\d{2}))?\s*(am|pm)?/);
        if (timeMatch) {
            let hours = parseInt(timeMatch[1], 10);
            const minutes = timeMatch[2] ? parseInt(timeMatch[2], 10) : 0;
            const ampm = timeMatch[3];
            
            if (ampm) {
                if (ampm === 'pm' && hours !== 12) hours += 12;
                if (ampm === 'am' && hours === 12) hours = 0;
            } else if (hours < 12 && input.includes('pm')) {
                hours += 12;
            }
            
            date.setHours(hours, minutes, 0, 0);
        }
        
        return date.toISOString();
    }
    
    // Handle "today"
    if (input.startsWith('today')) {
        date = new Date(now);
        date.setHours(0, 0, 0, 0);
        
        const timeMatch = input.match(/(\d{1,2})(?::(\d{2}))?\s*(am|pm)?/);
        if (timeMatch) {
            let hours = parseInt(timeMatch[1], 10);
            const minutes = timeMatch[2] ? parseInt(timeMatch[2], 10) : 0;
            const ampm = timeMatch[3];
            
            if (ampm) {
                if (ampm === 'pm' && hours !== 12) hours += 12;
                if (ampm === 'am' && hours === 12) hours = 0;
            }
            
            date.setHours(hours, minutes, 0, 0);
        }
        
        return date.toISOString();
    }
    
    // Handle "in X days/weeks/months"
    const inMatch = input.match(/in\s+(\d+)\s+(day|days|week|weeks|month|months)/);
    if (inMatch) {
        const amount = parseInt(inMatch[1], 10);
        const unit = inMatch[2];
        
        date = new Date(now);
        
        if (unit.startsWith('day')) {
            date.setDate(now.getDate() + amount);
        } else if (unit.startsWith('week')) {
            date.setDate(now.getDate() + (amount * 7));
        } else if (unit.startsWith('month')) {
            date.setMonth(now.getMonth() + amount);
        }
        
        date.setHours(0, 0, 0, 0);
        return date.toISOString();
    }
    
    // Handle "next [day of week]"
    const dayNames = ['sunday', 'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday'];
    const nextDayMatch = input.match(/next\s+(\w+)/);
    if (nextDayMatch) {
        const dayName = nextDayMatch[1];
        const dayIndex = dayNames.indexOf(dayName);
        
        if (dayIndex !== -1) {
            date = new Date(now);
            const daysUntil = (dayIndex - now.getDay() + 7) % 7 || 7;
            date.setDate(now.getDate() + daysUntil);
            date.setHours(0, 0, 0, 0);
            
            // Check for time
            const timeMatch = input.match(/(\d{1,2})(?::(\d{2}))?\s*(am|pm)?/);
            if (timeMatch) {
                let hours = parseInt(timeMatch[1], 10);
                const minutes = timeMatch[2] ? parseInt(timeMatch[2], 10) : 0;
                const ampm = timeMatch[3];
                
                if (ampm) {
                    if (ampm === 'pm' && hours !== 12) hours += 12;
                    if (ampm === 'am' && hours === 12) hours = 0;
                }
                
                date.setHours(hours, minutes, 0, 0);
            }
            
            return date.toISOString();
        }
    }
    
    // Try parsing with JavaScript's Date constructor
    const parsedDate = new Date(dateString);
    if (!isNaN(parsedDate.getTime())) {
        return parsedDate.toISOString();
    }
    
    return null;
}

/**
 * Format a date for display
 * @param {string} isoString - ISO 8601 date string
 * @returns {string} - Formatted date string
 */
function formatDate(isoString) {
    if (!isoString) return '';
    
    const date = new Date(isoString);
    if (isNaN(date.getTime())) return '';
    
    const now = new Date();
    const diffMs = date - now;
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
    
    // Format time
    const timeStr = date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
    
    // Relative dates for near future
    if (diffDays === 0) {
        return `Today at ${timeStr}`;
    } else if (diffDays === 1) {
        return `Tomorrow at ${timeStr}`;
    } else if (diffDays > 1 && diffDays <= 7) {
        const dayName = date.toLocaleDateString([], { weekday: 'long' });
        return `${dayName} at ${timeStr}`;
    } else {
        // Full date
        const dateStr = date.toLocaleDateString([], { month: 'short', day: 'numeric', year: date.getFullYear() !== now.getFullYear() ? 'numeric' : undefined });
        return `${dateStr} at ${timeStr}`;
    }
}

/**
 * Check if a date string is valid
 * @param {string} dateString - Date string to validate
 * @returns {boolean} - True if valid
 */
function isValidDate(dateString) {
    if (!dateString || !dateString.trim()) {
        return false;
    }
    return parseNaturalDate(dateString) !== null;
}

/**
 * Extract date/time information from todo text and return cleaned text with detected date
 * @param {string} text - Todo text that may contain date information
 * @returns {object} - { cleanedText: string, detectedDate: string|null }
 */
function extractDateFromText(text) {
    if (!text || !text.trim()) {
        return { cleanedText: text, detectedDate: null };
    }
    
    const originalText = text.trim();
    let cleanedText = originalText;
    let detectedDate = null;
    
    // Patterns to look for (order matters - more specific first)
    const datePatterns = [
        // "tomorrow at 3pm", "tomorrow at 3:30pm", "tomorrow"
        {
            pattern: /\b(tomorrow(?:\s+at\s+\d{1,2}(?::\d{2})?\s*(?:am|pm)?)?)\b/gi,
            parseFn: (match) => parseNaturalDate(match[1])
        },
        // "today at 5pm", "today"
        {
            pattern: /\b(today(?:\s+at\s+\d{1,2}(?::\d{2})?\s*(?:am|pm)?)?)\b/gi,
            parseFn: (match) => parseNaturalDate(match[1])
        },
        // "next friday", "next friday at 2pm", "next monday"
        {
            pattern: /\b(next\s+(?:monday|tuesday|wednesday|thursday|friday|saturday|sunday)(?:\s+at\s+\d{1,2}(?::\d{2})?\s*(?:am|pm)?)?)\b/gi,
            parseFn: (match) => parseNaturalDate(match[1])
        },
        // "in 3 days", "in 2 weeks"
        {
            pattern: /\b(in\s+\d+\s+(?:day|days|week|weeks|month|months))\b/gi,
            parseFn: (match) => parseNaturalDate(match[1])
        },
        // "on friday", "on friday at 3pm" (this week's or next week's)
        {
            pattern: /\b(on\s+(?:monday|tuesday|wednesday|thursday|friday|saturday|sunday)(?:\s+at\s+\d{1,2}(?::\d{2})?\s*(?:am|pm)?)?)\b/gi,
            parseFn: (match) => {
                // Determine if "on friday" means this week's or next week's friday
                const dayMatch = match[1].match(/on\s+(\w+)/);
                if (dayMatch) {
                    const dayNames = ['sunday', 'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday'];
                    const dayName = dayMatch[1].toLowerCase();
                    const dayIndex = dayNames.indexOf(dayName);
                    
                    if (dayIndex !== -1) {
                        const now = new Date();
                        const today = now.getDay();
                        const daysUntil = (dayIndex - today + 7) % 7;
                        // If it's today or already past this week, use next week's occurrence
                        const targetDays = daysUntil === 0 ? 7 : daysUntil;
                        
                        const date = new Date(now);
                        date.setDate(now.getDate() + targetDays);
                        date.setHours(0, 0, 0, 0);
                        
                        // Check for time specification
                        const timeMatch = match[1].match(/(\d{1,2})(?::(\d{2}))?\s*(am|pm)?/);
                        if (timeMatch) {
                            let hours = parseInt(timeMatch[1], 10);
                            const minutes = timeMatch[2] ? parseInt(timeMatch[2], 10) : 0;
                            const ampm = timeMatch[3];
                            
                            if (ampm) {
                                if (ampm === 'pm' && hours !== 12) hours += 12;
                                if (ampm === 'am' && hours === 12) hours = 0;
                            }
                            
                            date.setHours(hours, minutes, 0, 0);
                        }
                        
                        return date.toISOString();
                    }
                }
                return null;
            }
        },
        // Date formats: "March 15", "March 15, 2024", "03/15/2024", "2024-03-15"
        {
            pattern: /\b(\d{1,2}\/\d{1,2}\/\d{2,4}|\d{4}-\d{1,2}-\d{1,2}|(?:january|february|march|april|may|june|july|august|september|october|november|december)\s+\d{1,2}(?:,\s+\d{4})?)\b/gi,
            parseFn: (match) => {
                const parsed = parseNaturalDate(match[1]);
                // Only use if it's a future date (not just a year in the past)
                if (parsed) {
                    const date = new Date(parsed);
                    const now = new Date();
                    // Accept dates from today onwards (allow some past dates that might be typos for this year)
                    if (date >= now || (date.getFullYear() === now.getFullYear() && date >= new Date(now.getFullYear(), 0, 1))) {
                        return parsed;
                    }
                }
                return null;
            }
        },
        // Time patterns: "at 3pm", "at 3:30pm" (without a day, assume today if early in day, tomorrow if late)
        {
            pattern: /\bat\s+(\d{1,2}(?::\d{2})?\s*(?:am|pm))\b/gi,
            parseFn: (match) => {
                const now = new Date();
                const timeStr = match[1];
                // Parse time to see if it's today or tomorrow
                const timeMatch = timeStr.match(/(\d{1,2})(?::(\d{2}))?\s*(am|pm)/);
                if (timeMatch) {
                    let hours = parseInt(timeMatch[1], 10);
                    const ampm = timeMatch[3];
                    if (ampm === 'pm' && hours !== 12) hours += 12;
                    if (ampm === 'am' && hours === 12) hours = 0;
                    
                    // If it's before current time, assume tomorrow, otherwise today
                    const targetDate = new Date(now);
                    if (hours < now.getHours() || (hours === now.getHours() && now.getMinutes() > 0)) {
                        targetDate.setDate(targetDate.getDate() + 1);
                    }
                    targetDate.setHours(hours, timeMatch[2] ? parseInt(timeMatch[2], 10) : 0, 0, 0);
                    return targetDate.toISOString();
                }
                return null;
            }
        }
    ];
    
    // Try each pattern and use the first match that successfully parses
    for (const { pattern, parseFn } of datePatterns) {
        // Reset regex lastIndex to ensure we start from the beginning
        pattern.lastIndex = 0;
        const match = pattern.exec(originalText);
        if (match) {
            const parsedDate = parseFn(match);
            if (parsedDate) {
                detectedDate = parsedDate;
                // Remove the matched date expression from the text
                // Reset pattern again for replace
                pattern.lastIndex = 0;
                cleanedText = originalText.replace(pattern, '').replace(/\s+/g, ' ').trim();
                // Remove leading/trailing punctuation that might be left behind
                cleanedText = cleanedText.replace(/^[,\s]+|[,\s]+$/g, '');
                break;
            }
        }
    }
    
    return { cleanedText, detectedDate };
}
