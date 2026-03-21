#!/usr/bin/env python3
"""
detect-triggers.py — Analyze conversation for auto-memory triggers
Detects: build success, error resolution, architecture decisions, user preferences
"""

import json
import re
import sys
from datetime import datetime
from typing import List, Dict, Optional, Tuple

class TriggerDetector:
    """Detects auto-memory triggers from conversation context"""
    
    # Build/test command patterns
    BUILD_PATTERNS = [
        r'(?:make|npm|yarn|cargo|go|gradle|maven)\s+(?:build|compile|bundle)',
        r'(?:go|cargo)\s+build',
        r'npm\s+run\s+build',
        r'yarn\s+build',
        r'make\s+build',
    ]
    
    TEST_PATTERNS = [
        r'(?:make|npm|yarn|cargo|go|gradle|maven)\s+test',
        r'go\s+test',
        r'npm\s+(?:test|t)',
        r'yarn\s+test',
        r'pytest',
        r'jest',
        r'make\s+test',
    ]
    
    DEPLOY_PATTERNS = [
        r'(?:make|npm|yarn)\s+deploy',
        r'docker\s+push',
        r'kubectl\s+apply',
        r'vercel\s+--prod',
        r'netlify\s+deploy',
    ]
    
    # Success indicators
    SUCCESS_PATTERNS = [
        r'success(?:ful)?',
        r'passed',
        r'built',
        r'compiled',
        r'✓',
        r'✔',
        r'done',
        r'finished',
        r'complete',
        r'\[100%\]',
        r'0 errors?',
        r'all tests passed',
    ]
    
    # Error patterns
    ERROR_PATTERNS = [
        r'error[:：]',
        r'failed[:：]',
        r'exception[:：]',
        r'cannot\s+',
        r'unable\s+to\s+',
        r'module\s+not\s+found',
        r'package\s+not\s+found',
        r'undefined\s+',
        r'permission\s+denied',
        r'no\s+such\s+file',
        r'ENOENT',
    ]
    
    # Solution indicators
    SOLUTION_PATTERNS = [
        r'(?:try|run|use)\s+[`\'"]?[\w\-\.]+[`\'"]?',
        r'(?:fixed|resolved|solved|works?\s+now)',
        r'(?:solution|fix):\s*',
        r'you\s+(?:need\s+to\s+|should\s+)?(?:run|execute|use)',
        r'(?:the\s+)?(?:problem|issue)\s+(?:is|was)',
    ]
    
    # Decision patterns
    DECISION_PATTERNS = [
        r'(?:let\'s|we\s+(?:will|should)|I\s+(?:will|prefer))\s+(?:use|go\s+with|choose)',
        r'(?:decision|decided):\s*',
        r'(?:chose|selected|picked)\s+',
        r'(?:will\s+use|going\s+with)\s+',
        r'(?:决定|选择|使用)',
    ]
    
    # Preference patterns
    PREFERENCE_PATTERNS = [
        r'(?:always|never)\s+',
        r'I\s+prefer\s+',
        r'(?:please\s+)?make\s+sure\s+to\s+',
        r'(?:from\s+now\s+on|going\s+forward)',
        r'(?:偏好|喜欢|希望)',
    ]
    
    def __init__(self):
        self.last_error = None
        self.last_command = None
        
    def detect_triggers(self, messages: List[Dict]) -> List[Dict]:
        """
        Analyze conversation messages and return detected triggers.
        
        Returns list of triggers, each with:
        - type: 'build_command', 'error_solution', 'decision', 'preference'
        - data: dict with relevant information
        """
        triggers = []
        
        for i, msg in enumerate(messages):
            role = msg.get('role', '')
            content = msg.get('content', '')
            
            if isinstance(content, list):
                # Extract text from structured content
                content = ' '.join(
                    block.get('text', '')
                    for block in content
                    if isinstance(block, dict) and block.get('type') == 'text'
                )
            
            content_lower = content.lower()
            
            # Check for build/test/deploy commands
            if role == 'user':
                triggers.extend(self._detect_commands(content, content_lower))
            
            # Check for command success
            if role == 'assistant':
                for pattern in self.BUILD_PATTERNS + self.TEST_PATTERNS + self.DEPLOY_PATTERNS:
                    if re.search(pattern, content_lower):
                        # Check if succeeded
                        for success_pattern in self.SUCCESS_PATTERNS:
                            if re.search(success_pattern, content_lower):
                                triggers.extend(self._extract_command_success(content))
                                break
            
            # Check for errors
            if role in ['user', 'assistant']:
                for error_pattern in self.ERROR_PATTERNS:
                    if re.search(error_pattern, content_lower):
                        self.last_error = self._extract_error(content)
                        break
            
            # Check for solutions
            if self.last_error and role == 'assistant':
                for solution_pattern in self.SOLUTION_PATTERNS:
                    if re.search(solution_pattern, content_lower):
                        triggers.append({
                            'type': 'error_solution',
                            'data': self.last_error
                        })
                        self.last_error = None
                        break
            
            # Check for decisions
            if role == 'user':
                for decision_pattern in self.DECISION_PATTERNS:
                    if re.search(decision_pattern, content_lower):
                        triggers.append({
                            'type': 'decision',
                            'data': self._extract_decision(content)
                        })
                        break
            
            # Check for preferences
            if role == 'user':
                for pref_pattern in self.PREFERENCE_PATTERNS:
                    if re.search(pref_pattern, content_lower):
                        triggers.append({
                            'type': 'preference',
                            'data': self._extract_preference(content)
                        })
                        break
        
        return triggers
    
    def _detect_commands(self, content: str, content_lower: str) -> List[Dict]:
        """Detect build/test/deploy commands in content"""
        triggers = []
        
        # Check each command category
        for pattern in self.BUILD_PATTERNS:
            match = re.search(pattern, content_lower)
            if match:
                self.last_command = ('Build', match.group(0))
                break
        
        for pattern in self.TEST_PATTERNS:
            match = re.search(pattern, content_lower)
            if match:
                self.last_command = ('Test', match.group(0))
                break
        
        for pattern in self.DEPLOY_PATTERNS:
            match = re.search(pattern, content_lower)
            if match:
                self.last_command = ('Deploy', match.group(0))
                break
        
        return triggers
    
    def _extract_command_success(self, content: str) -> List[Dict]:
        """Extract successful command execution"""
        triggers = []
        
        if self.last_command:
            category, command = self.last_command
            triggers.append({
                'type': 'build_command',
                'data': {
                    'command': command,
                    'category': category,
                    'description': f'{category} command'
                }
            })
            self.last_command = None
        
        return triggers
    
    def _extract_error(self, content: str) -> Dict:
        """Extract error pattern from content"""
        # Extract first line or first 100 chars
        lines = content.strip().split('\n')
        error_msg = lines[0] if lines else content[:100]
        
        return {
            'error': error_msg[:100],  # Limit error message length
            'context': content[:200]   # Include some context
        }
    
    def _extract_decision(self, content: str) -> Dict:
        """Extract architecture decision from content"""
        # Extract the decision statement
        sentences = re.split(r'[.!?。！？]', content)
        decision_text = sentences[0] if sentences else content[:200]
        
        # Try to extract rationale (next sentence or rest of content)
        rationale = ' '.join(sentences[1:3]) if len(sentences) > 1 else ''
        rationale = rationale[:200] if rationale else 'Not specified'
        
        return {
            'decision': decision_text[:200],
            'rationale': rationale,
            'date': datetime.now().strftime('%Y-%m-%d')
        }
    
    def _extract_preference(self, content: str) -> Dict:
        """Extract user preference from content"""
        # Extract first sentence
        sentences = re.split(r'[.!?。！？]', content)
        preference = sentences[0] if sentences else content[:200]
        
        return {
            'preference': preference[:200],
            'context': content[:100]
        }


def main():
    """Main entry point - reads JSON from stdin"""
    try:
        # Read JSON input from stdin
        data = json.load(sys.stdin)
        
        # Extract messages
        messages = data.get('messages', data.get('transcript', []))
        
        # Detect triggers
        detector = TriggerDetector()
        triggers = detector.detect_triggers(messages)
        
        # Output triggers as JSON
        output = {
            'triggers': triggers,
            'count': len(triggers)
        }
        
        print(json.dumps(output, indent=2, ensure_ascii=False))
        
    except Exception as e:
        print(json.dumps({'error': str(e)}), file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
