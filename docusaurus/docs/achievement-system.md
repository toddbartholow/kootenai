# Achievement and Badge System

## Overview

The achievement system provides gamification features that reward students for completing labs, achieving high scores, maintaining streaks, and other accomplishments. Achievements motivate students, track progress, and add a competitive element to learning.

## Architecture

### Components

1. **Achievement Models** (`internal/models/achievement.go`)
   - Achievement definitions
   - User achievement records
   - Progress tracking
   - Achievement criteria

2. **Achievement Repository** (`internal/database/repositories/achievement_repo.go`)
   - CRUD operations for achievements
   - User achievement tracking
   - Progress persistence
   - Statistics and summaries

3. **Achievement Service** (`internal/achievements/service.go`)
   - Business logic for awarding achievements
   - Criteria evaluation
   - Progress calculation
   - Automatic awarding on lab submission

4. **API Handlers** (`internal/server/achievement_handlers.go`)
   - REST endpoints for achievements
   - User achievement queries
   - Leaderboard support

5. **Database Schema** (`migrations/009_achievements.sql`)
   - achievements table
   - user_achievements table
   - achievement_progress table

## Achievement Types

### 1. Lab Completion (`lab_completion`)
Awarded for completing specific numbers of labs.

**Examples**:
- First Steps: Complete your first lab (10 points, Bronze)
- Lab Veteran: Complete 10 labs (50 points, Silver)
- Lab Master: Complete 25 labs (150 points, Gold)
- Lab Legend: Complete 50 labs (300 points, Platinum)

**Criteria**:
```json
{
  "totalLabs": 10
}
```

### 2. Perfect Score (`perfect_score`)
Awarded for achieving 100% on labs.

**Examples**:
- Perfectionist: Earn a perfect score on any lab (25 points, Silver)
- Flawless Five: Earn perfect scores on 5 different labs (100 points, Gold)

**Criteria**:
```json
{
  "requirePerfect": true,
  "totalLabs": 1
}
```

### 3. Speed (`speed`)
Awarded for completing labs quickly.

**Examples**:
- Speed Demon: Complete a lab in under 15 minutes (75 points, Gold)

**Criteria**:
```json
{
  "maxDurationMins": 15
}
```

### 4. Streak (`streak`)
Awarded for consecutive successful completions.

**Examples**:
- On Fire: Complete 3 labs in a row (30 points, Bronze)
- Unstoppable: Complete 7 labs in a row (75 points, Silver)

**Criteria**:
```json
{
  "streakCount": 3,
  "streakType": "any"
}
```

### 5. Category (`category`)
Awarded for completing all labs in a category.

**Examples**:
- Security Expert: Complete all security labs (100 points, Gold)
- Network Ninja: Complete all networking labs (100 points, Gold)

**Criteria**:
```json
{
  "labCategory": "security"
}
```

### 6. Milestone (`milestone`)
Awarded for reaching milestones.

**Examples**:
- 100 Points: Earn 100 total points (milestone achievement)

**Criteria**:
```json
{
  "totalPoints": 100
}
```

### 7. Special (`special`)
Awarded for special circumstances.

**Examples**:
- Early Bird: Complete a lab before 6 AM (20 points, Bronze)
- Night Owl: Complete a lab after 10 PM (20 points, Bronze)

**Criteria**:
```json
{
  "customRule": "early_morning"
}
```

## Achievement Tiers

Achievements have tiers indicating rarity/difficulty:

| Tier | Description | Typical Point Range |
|------|-------------|---------------------|
| **Bronze** | Common, easy achievements | 5-30 points |
| **Silver** | Uncommon, moderate achievements | 25-75 points |
| **Gold** | Rare, difficult achievements | 75-150 points |
| **Platinum** | Very rare, very difficult | 150-300 points |
| **Diamond** | Legendary, extremely rare | 300+ points |

## Database Schema

### achievements
```sql
CREATE TABLE achievements (
    id VARCHAR(128) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,
    tier VARCHAR(50) NOT NULL DEFAULT 'bronze',
    icon_url VARCHAR(512),
    points INT NOT NULL DEFAULT 0,
    is_secret BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    criteria JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### user_achievements
```sql
CREATE TABLE user_achievements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    achievement_id VARCHAR(128) NOT NULL REFERENCES achievements(id),
    earned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    session_id VARCHAR(128),
    progress DECIMAL(5,2) DEFAULT 100.00,
    notified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, achievement_id)
);
```

### achievement_progress
```sql
CREATE TABLE achievement_progress (
    user_id UUID NOT NULL REFERENCES users(id),
    achievement_id VARCHAR(128) NOT NULL REFERENCES achievements(id),
    progress DECIMAL(5,2) NOT NULL DEFAULT 0.00,
    current_value INT NOT NULL DEFAULT 0,
    required_value INT NOT NULL DEFAULT 1,
    metadata JSONB DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(user_id, achievement_id)
);
```

## API Endpoints

### List All Achievements
```http
GET /api/v1/achievements
GET /api/v1/achievements?type=perfect_score
GET /api/v1/achievements?tier=gold
GET /api/v1/achievements?active=true
```

**Response**:
```json
{
  "achievements": [
    {
      "id": "first-lab",
      "name": "First Steps",
      "description": "Complete your first lab",
      "type": "lab_completion",
      "tier": "bronze",
      "points": 10,
      "isSecret": false,
      "isActive": true,
      "criteria": {"totalLabs": 1}
    }
  ],
  "count": 1
}
```

### Get Achievement Details
```http
GET /api/v1/achievements/{achievementID}
```

### Get User Achievements
```http
GET /api/v1/users/{userID}/achievements
```

**Response**:
```json
{
  "achievements": [
    {
      "id": "first-lab",
      "name": "First Steps",
      "description": "Complete your first lab",
      "type": "lab_completion",
      "tier": "bronze",
      "points": 10,
      "earned": true,
      "earnedAt": "2025-12-25T10:30:00Z",
      "progress": {
        "progress": 100.0,
        "current": 1,
        "required": 1
      }
    },
    {
      "id": "lab-veteran",
      "name": "Lab Veteran",
      "description": "Complete 10 labs",
      "type": "milestone",
      "tier": "silver",
      "points": 50,
      "earned": false,
      "progress": {
        "progress": 30.0,
        "current": 3,
        "required": 10
      }
    }
  ],
  "count": 2
}
```

### Get User Achievement Summary
```http
GET /api/v1/users/{userID}/achievements/summary
```

**Response**:
```json
{
  "userId": "user-123",
  "totalEarned": 5,
  "totalPoints": 150,
  "totalAvailable": 16,
  "completionPercent": 31.25,
  "recentAchievements": [
    {
      "id": "ua-abc",
      "achievementId": "perfectionist",
      "earnedAt": "2025-12-25T15:00:00Z",
      "sessionId": "session-xyz"
    }
  ],
  "rarityBreakdown": {
    "bronze": 3,
    "silver": 1,
    "gold": 1
  },
  "typeBreakdown": {
    "lab_completion": 3,
    "perfect_score": 1,
    "speed": 1
  }
}
```

### Get Recent Achievements (Leaderboard)
```http
GET /api/v1/achievements/recent
```

Returns recently earned achievements across all users (last 10).

## Integration with Lab Submission

When a student submits a lab (`POST /api/v1/sessions/{sessionID}/submit`), the achievement service automatically:

1. Checks all active achievements
2. Evaluates criteria based on the session results
3. Awards achievements that are newly earned
4. Updates progress for in-progress achievements
5. Returns newly earned achievements in the submission response

**Example Submission Response**:
```json
{
  "sessionId": "session-123",
  "status": "submitted",
  "earnedPoints": 100,
  "maxPoints": 100,
  "percentage": 100.0,
  "passed": true,
  "achievements": [
    {
      "id": "ua-new123",
      "achievementId": "perfectionist",
      "earnedAt": "2025-12-25T15:00:00Z",
      "sessionId": "session-123"
    },
    {
      "id": "ua-new456",
      "achievementId": "first-lab",
      "earnedAt": "2025-12-25T15:00:00Z",
      "sessionId": "session-123"
    }
  ]
}
```

## Seeded Achievements

The system seeds 16 default achievements on initial migration:

| ID | Name | Type | Tier | Points |
|----|------|------|------|--------|
| first-lab | First Steps | lab_completion | bronze | 10 |
| lab-veteran | Lab Veteran | milestone | silver | 50 |
| lab-master | Lab Master | milestone | gold | 150 |
| lab-legend | Lab Legend | milestone | platinum | 300 |
| perfectionist | Perfectionist | perfect_score | silver | 25 |
| flawless-five | Flawless Five | perfect_score | gold | 100 |
| speed-demon | Speed Demon | speed | gold | 75 |
| on-fire | On Fire! | streak | bronze | 30 |
| unstoppable | Unstoppable | streak | silver | 75 |
| security-expert | Security Expert | category | gold | 100 |
| network-ninja | Network Ninja | category | gold | 100 |
| brave-beginner | Brave Beginner | lab_completion | bronze | 5 |
| advanced-achiever | Advanced Achiever | lab_completion | silver | 40 |
| expert-elite | Expert Elite | lab_completion | gold | 100 |
| early-bird | Early Bird | special | bronze | 20 |
| night-owl | Night Owl | special | bronze | 20 |

## Adding Custom Achievements

Create custom achievements via SQL:

```sql
INSERT INTO achievements (id, name, description, type, tier, points, criteria)
VALUES (
    'custom-achievement',
    'Custom Achievement',
    'Description of what this achievement requires',
    'lab_completion',
    'gold',
    150,
    '{"totalLabs": 20}'::jsonb
);
```

Or programmatically:

```go
achievement := &models.Achievement{
    ID:          "custom-achievement",
    Name:        "Custom Achievement",
    Description: "Complete 20 security labs",
    Type:        models.AchievementTypeLabCompletion,
    Tier:        models.AchievementTierGold,
    Points:      150,
    IsActive:    true,
    Criteria: models.AchievementCriteria{
        TotalLabs:     20,
        LabCategory:   "security",
    },
}

err := achievementRepo.CreateAchievement(ctx, achievement)
```

## Secret Achievements

Achievements can be marked as secret (`is_secret = true`). Secret achievements:
- Are hidden from users until earned
- Don't show in the available achievements list
- Surprise users when earned
- Great for easter eggs or rare accomplishments

## Progress Tracking

The system tracks progress towards achievements even before they're earned:

- **Current Value**: How many labs completed, current streak, etc.
- **Required Value**: How many needed to earn
- **Progress Percentage**: (current / required) * 100
- **Metadata**: Additional tracking data (e.g., which labs, timestamps)

## Notifications

The `notified` field tracks whether the user has been notified about earning an achievement:

1. Achievement earned → `notified = false`
2. Frontend queries `/users/{userID}/achievements`
3. Shows notification to user
4. Frontend calls API to mark notified
5. Updates `notified = true`

## Leaderboard Support

Use the achievement system to build leaderboards:

**Total Points**:
```sql
SELECT u.username, SUM(a.points) as total_points
FROM user_achievements ua
JOIN achievements a ON ua.achievement_id = a.id
JOIN users u ON ua.user_id = u.id
GROUP BY u.id, u.username
ORDER BY total_points DESC
LIMIT 10;
```

**Recent Achievements**:
```http
GET /api/v1/achievements/recent
```

**Most Achievements Earned**:
```sql
SELECT u.username, COUNT(ua.id) as achievement_count
FROM user_achievements ua
JOIN users u ON ua.user_id = u.id
GROUP BY u.id, u.username
ORDER BY achievement_count DESC
LIMIT 10;
```

## Future Enhancements

### 1. Achievement Categories
Group achievements by theme (Beginner, Expert, Special, etc.)

### 2. Achievement Chains
Require specific achievements before others can be earned

### 3. Time-Limited Achievements
Seasonal or event-based achievements that expire

### 4. Team Achievements
Achievements earned by groups completing collaborative labs

### 5. Achievement Sharing
Share earned achievements on social media or lab profiles

### 6. Custom Icons
Support for custom SVG or image icons for achievements

### 7. Achievement Store
Spend achievement points on perks (extended lab time, hints, etc.)

## Testing

### Unit Tests
Test achievement criteria evaluation:

```go
func TestCheckPerfectScoreCriteria(t *testing.T) {
    service := achievements.NewService(repo, sessionRepo, logger)

    session := &models.Session{
        Percentage: 100.0,
        Passed:     true,
    }

    achievement := &models.Achievement{
        Type: models.AchievementTypePerfectScore,
        Criteria: models.AchievementCriteria{
            RequirePerfect: true,
        },
    }

    met, progress, err := service.checkCriteria(ctx, userID, session, achievement)

    assert.NoError(t, err)
    assert.True(t, met)
    assert.Equal(t, 100.0, progress.Progress)
}
```

### Integration Tests
Test full flow from lab submission to achievement award:

```bash
# Submit a lab
curl -X POST http://localhost:8080/api/v1/sessions/{sessionId}/submit

# Check achievements were awarded
curl http://localhost:8080/api/v1/users/{userId}/achievements

# Verify summary updated
curl http://localhost:8080/api/v1/users/{userId}/achievements/summary
```

## Performance Considerations

- **Indexing**: Critical for user achievement queries
- **Caching**: Cache frequently accessed achievements and summaries
- **Batch Processing**: Process achievement checks asynchronously for large user bases
- **Pagination**: Implement pagination for large achievement lists

## Related Documentation

- [Database Schema](./reference/database-schema.md)
- [API Reference](./api/reference.md)
