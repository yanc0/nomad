/* eslint-disable ember/avoid-leaking-state-in-ember-objects */
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';
import setupAbility from 'nomad-ui/tests/helpers/setup-ability';

module('Unit | Ability | variable', function (hooks) {
  setupTest(hooks);
  setupAbility('variable')(hooks);
  hooks.beforeEach(function () {
    const mockSystem = Service.extend({
      features: [],
    });

    this.owner.register('service:system', mockSystem);
  });

  module('#list', function () {
    test('it does not permit listing variables by default', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canList);
    });

    test('it does not permit listing variables when token type is client', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canList);
    });

    test('it permits listing variables when token type is management', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'management' },
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canList);
    });

    test('it permits listing variables when token has SecureVariables with list capabilities in its rules', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: 'default',
                  Capabilities: [],
                  SecureVariables: {
                    'Path "*"': {
                      Capabilities: ['list'],
                    },
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canList);
    });

    test('it permits listing variables when token has SecureVariables alone in its rules', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: 'default',
                  Capabilities: [],
                  SecureVariables: {},
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canList);
    });
  });

  module('#create', function () {
    test('it does not permit creating variables by default', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canCreate);
    });

    test('it permits creating variables when token type is management', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'management' },
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canCreate);
    });

    test('it permits creating variables when acl is disabled', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: false,
        selfToken: { type: 'client' },
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canCreate);
    });

    test('it permits creating variables when token has SecureVariables with create capabilities in its rules', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: 'default',
                  Capabilities: [],
                  SecureVariables: {
                    'Path "*"': {
                      Capabilities: ['create'],
                    },
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canCreate);
    });

    test('it handles namespace matching', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: 'default',
                  Capabilities: [],
                  SecureVariables: {
                    'Path "foo/bar"': {
                      Capabilities: ['list'],
                    },
                  },
                },
                {
                  Name: 'pablo',
                  Capabilities: [],
                  SecureVariables: {
                    'Path "foo/bar"': {
                      Capabilities: ['create'],
                    },
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);
      this.ability.path = 'foo/bar';
      this.ability.namespace = 'pablo';

      assert.ok(this.ability.canCreate);
    });
  });

  module('#_nearestMatchingPath', function () {
    test('returns capabilities for an exact path match', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: 'default',
                  Capabilities: [],
                  SecureVariables: {
                    'Path "foo"': {
                      Capabilities: ['create'],
                    },
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);
      const path = 'foo';

      const nearestMatchingPath = this.ability._nearestMatchingPath(path);

      assert.equal(
        nearestMatchingPath,
        'foo',
        'It should return the exact path match.'
      );
    });

    test('returns capabilities for the nearest ancestor if no exact match', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: 'default',
                  Capabilities: [],
                  SecureVariables: {
                    'Path "foo/*"': {
                      Capabilities: ['create'],
                    },
                    'Path "foo/bar/*"': {
                      Capabilities: ['create'],
                    },
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);
      const path = 'foo/bar/baz';

      const nearestMatchingPath = this.ability._nearestMatchingPath(path);

      assert.equal(
        nearestMatchingPath,
        'foo/bar/*',
        'It should return the nearest ancestor matching path.'
      );
    });

    test('handles wildcard prefix matches', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: 'default',
                  Capabilities: [],
                  SecureVariables: {
                    'Path "foo/*"': {
                      Capabilities: ['create'],
                    },
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);
      const path = 'foo/bar/baz';

      const nearestMatchingPath = this.ability._nearestMatchingPath(path);

      assert.equal(
        nearestMatchingPath,
        'foo/*',
        'It should handle wildcard glob prefixes.'
      );
    });

    test('handles wildcard suffix matches', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: 'default',
                  Capabilities: [],
                  SecureVariables: {
                    'Path "*/bar"': {
                      Capabilities: ['create'],
                    },
                    'Path "*/bar/baz"': {
                      Capabilities: ['create'],
                    },
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);
      const path = 'foo/bar/baz';

      const nearestMatchingPath = this.ability._nearestMatchingPath(path);

      assert.equal(
        nearestMatchingPath,
        '*/bar/baz',
        'It should return the nearest ancestor matching path.'
      );
    });

    test('prioritizes wildcard suffix matches over wildcard prefix matches', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: 'default',
                  Capabilities: [],
                  SecureVariables: {
                    'Path "*/bar"': {
                      Capabilities: ['create'],
                    },
                    'Path "foo/*"': {
                      Capabilities: ['create'],
                    },
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);
      const path = 'foo/bar/baz';

      const nearestMatchingPath = this.ability._nearestMatchingPath(path);

      assert.equal(
        nearestMatchingPath,
        'foo/*',
        'It should prioritize suffix glob wildcard of prefix glob wildcard.'
      );
    });

    test('defaults to the glob path if there is no exact match or wildcard matches', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: 'default',
                  Capabilities: [],
                  SecureVariables: {
                    'Path "*"': {
                      Capabilities: ['create'],
                    },
                    'Path "foo"': {
                      Capabilities: ['create'],
                    },
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);
      const path = 'foo/bar/baz';

      const nearestMatchingPath = this.ability._nearestMatchingPath(path);

      assert.equal(
        nearestMatchingPath,
        '*',
        'It should default to glob wildcard if no matches.'
      );
    });
  });

  module('#_doesMatchPattern', function () {
    const edgeCaseTest = 'this is a ϗѾ test';

    module('base cases', function () {
      test('it handles an empty pattern', function (assert) {
        // arrange
        const pattern = '';
        const emptyPath = '';
        const nonEmptyPath = 'a';

        // act
        const matchingResult = this.ability._doesMatchPattern(
          pattern,
          emptyPath
        );
        const nonMatchingResult = this.ability._doesMatchPattern(
          pattern,
          nonEmptyPath
        );

        // assert
        assert.ok(matchingResult, 'Empty pattern should match empty path');
        assert.notOk(
          nonMatchingResult,
          'Empty pattern should not match non-empty path'
        );
      });

      test('it handles an empty path', function (assert) {
        // arrange
        const emptyPath = '';
        const emptyPattern = '';
        const nonEmptyPattern = 'a';

        // act
        const matchingResult = this.ability._doesMatchPattern(
          emptyPattern,
          emptyPath
        );
        const nonMatchingResult = this.ability._doesMatchPattern(
          nonEmptyPattern,
          emptyPath
        );

        // assert
        assert.ok(matchingResult, 'Empty path should match empty pattern');
        assert.notOk(
          nonMatchingResult,
          'Empty path should not match non-empty pattern'
        );
      });

      test('it handles a pattern without a glob', function (assert) {
        // arrange
        const path = '/foo';
        const matchingPattern = '/foo';
        const nonMatchingPattern = '/bar';

        // act
        const matchingResult = this.ability._doesMatchPattern(
          matchingPattern,
          path
        );
        const nonMatchingResult = this.ability._doesMatchPattern(
          nonMatchingPattern,
          path
        );

        // assert
        assert.ok(matchingResult, 'Matches path correctly.');
        assert.notOk(nonMatchingResult, 'Does not match non-matching path.');
      });

      test('it handles a pattern that is a lone glob', function (assert) {
        // arrange
        const path = '/foo';
        const glob = '*';

        // act
        const matchingResult = this.ability._doesMatchPattern(glob, path);

        // assert
        assert.ok(matchingResult, 'Matches glob.');
      });

      test('it matches on leading glob', function (assert) {
        // arrange
        const pattern = '*bar';
        const matchingPath = 'footbar';
        const nonMatchingPath = 'rockthecasba';

        // act
        const matchingResult = this.ability._doesMatchPattern(
          pattern,
          matchingPath
        );
        const nonMatchingResult = this.ability._doesMatchPattern(
          pattern,
          nonMatchingPath
        );

        // assert
        assert.ok(
          matchingResult,
          'Correctly matches when leading glob and matching path.'
        );
        assert.notOk(
          nonMatchingResult,
          'Does not match when leading glob and non-matching path.'
        );
      });

      test('it matches on trailing glob', function (assert) {
        // arrange
        const pattern = 'foo*';
        const matchingPath = 'footbar';
        const nonMatchingPath = 'bar';

        // act
        const matchingResult = this.ability._doesMatchPattern(
          pattern,
          matchingPath
        );
        const nonMatchingResult = this.ability._doesMatchPattern(
          pattern,
          nonMatchingPath
        );

        // assert
        assert.ok(matchingResult, 'Correctly matches on trailing glob.');
        assert.notOk(
          nonMatchingResult,
          'Does not match on trailing glob if pattern does not match.'
        );
      });

      test('it matches when glob is in middle', function (assert) {
        // arrange
        const pattern = 'foo*bar';
        const matchingPath = 'footbar';
        const nonMatchingPath = 'footba';

        // act
        const matchingResult = this.ability._doesMatchPattern(
          pattern,
          matchingPath
        );
        const nonMatchingResult = this.ability._doesMatchPattern(
          pattern,
          nonMatchingPath
        );

        // assert
        assert.ok(
          matchingResult,
          'Correctly matches on glob in middle of path.'
        );
        assert.notOk(
          nonMatchingResult,
          'Does not match on glob in middle of path if not full pattern match.'
        );
      });
    });

    module('matching edge cases', function () {
      test('it matches when string is between globs', function (assert) {
        // arrange
        const pattern = '*is *';

        // act
        const result = this.ability._doesMatchPattern(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles many non-consective globs', function (assert) {
        // arrange
        const pattern = '*is*a*';

        // act
        const result = this.ability._doesMatchPattern(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles double globs', function (assert) {
        // arrange
        const pattern = '**test**';

        // act
        const result = this.ability._doesMatchPattern(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles many consecutive globs', function (assert) {
        // arrange
        const pattern = '**is**a***test*';

        // act
        const result = this.ability._doesMatchPattern(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles white space between globs', function (assert) {
        // arrange
        const pattern = '* *';

        // act
        const result = this.ability._doesMatchPattern(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles a pattern of only globs', function (assert) {
        // arrange
        const pattern = '**********';

        // act
        const result = this.ability._doesMatchPattern(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles unicode characters', function (assert) {
        // arrange
        const pattern = `*Ѿ*`;

        // act
        const result = this.ability._doesMatchPattern(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });

      test('it handles mixed ASCII codes', function (assert) {
        // arrange
        const pattern = `*is a ϗѾ *`;

        // act
        const result = this.ability._doesMatchPattern(pattern, edgeCaseTest);

        // assert
        assert.ok(result);
      });
    });

    module('non-matching edge cases', function () {
      const failingCases = [
        {
          case: 'test*',
          message: 'Implicit substring match',
        },
        {
          case: '*is',
          message: 'Parial match',
        },
        {
          case: '*no*',
          message: 'Globs without match between them',
        },
        {
          case: ' ',
          message: 'Plain white space',
        },
        {
          case: '* ',
          message: 'Trailing white space',
        },
        {
          case: ' *',
          message: 'Leading white space',
        },
        {
          case: '*ʤ*',
          message: 'Non-matching unicode',
        },
        {
          case: 'this*this is a test',
          message: 'Repeated prefix',
        },
      ];

      failingCases.forEach(({ case: failingPattern, message }) => {
        test('should fail the specified cases', function (assert) {
          const result = this.ability._doesMatchPattern(
            failingPattern,
            edgeCaseTest
          );
          assert.notOk(result, `${message} should not match.`);
        });
      });
    });
  });
});
